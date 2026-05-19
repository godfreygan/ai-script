package server

import (
	"context"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/godfreygan/ai-script/backend/docs"
	"github.com/godfreygan/ai-script/backend/internal/conf"
	"github.com/godfreygan/ai-script/backend/internal/handler"
	"github.com/godfreygan/ai-script/backend/internal/middleware"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/jwt"
	"github.com/godfreygan/ai-script/backend/pkg/metrics"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/godfreygan/ai-script/backend/pkg/storage"
	"github.com/godfreygan/ai-script/backend/pkg/ws"
	"github.com/casbin/casbin/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type App struct {
	cfg      *conf.Config
	log      *zap.Logger
	router   *gin.Engine
	db       *gorm.DB
	rdb      *redis.Client
	enforcer *casbin.Enforcer
	services *service.Services
	hub      *ws.Hub
	store    storage.Storage
	cancel   context.CancelFunc
}

func NewApp(
	cfg *conf.Config,
	log *zap.Logger,
	db *gorm.DB,
	rdb *redis.Client,
	enforcer *casbin.Enforcer,
	jwtMgr *jwt.Manager,
	repos *repo.Repositories,
	services *service.Services,
	hub *ws.Hub,
	store storage.Storage,
	handlers *handler.Handlers,
) (*App, error) {
	// 启动时强制校验配置,防止默认值/弱密钥进入生产
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// 修复 P0 F4 — 迁移模式/源/DSN 从配置读取
	if cfg.Migrate.Mode != "off" {
		migCtx, mcancel := context.WithTimeout(context.Background(), 60*time.Second)
		migDSN := cfg.Migrate.DSN
		if migDSN == "" {
			migDSN = cfg.MySQL.DSN
		}
		if err := repos.Migrate(migCtx, cfg.Migrate.Mode, cfg.Migrate.Source, migDSN); err != nil {
			log.Warn("auto-migrate failed", zap.Error(err))
		}
		mcancel()
	}

	// 启动时同步 Casbin 策略 + 加载所有 enabled 模型 adapter
	ctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ccancel()
	if err := services.Role.SyncCasbin(ctx); err != nil {
		log.Warn("sync casbin failed", zap.Error(err))
	}
	if err := services.Model.LoadAllAdapters(ctx); err != nil {
		log.Warn("load adapters failed", zap.Error(err))
	}

	hubCtx, cancel := context.WithCancel(context.Background())
	go hub.Run(hubCtx)

	app := &App{cfg: cfg, log: log, db: db, rdb: rdb, enforcer: enforcer, services: services, hub: hub, store: store, cancel: cancel}
	app.router = newRouter(cfg, log, jwtMgr, enforcer, handlers, store, db, rdb)
	return app, nil
}

func (a *App) Router() *gin.Engine { return a.router }

// Server 返回配置好超时与安全参数的 http.Server，供外部启动与优雅关闭。
//
// HTTP/2 说明：当使用 TLS 时，Go 标准库 http.Server 默认启用 HTTP/2（通过
// golang.org/x/net/http2 自动协商）。非 TLS 场景下仅在非生产环境通过 h2c
// 支持 HTTP/2，方便本地调试与内网测试。
func (a *App) Server() *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler: a.router,
		// ReadHeaderTimeout 防御 slowloris：攻击者缓慢发送 HTTP 头，占用连接不释放。
		// 5s 足够穿越普通 NAT/移动网络，同时限制恶意连接生命周期。
		ReadHeaderTimeout: 5 * time.Second,
		// ReadTimeout 防御完整请求体慢速读取攻击，给正常表单/JSON 上传留有余量。
		ReadTimeout: 10 * time.Second,
		// WriteTimeout 防止响应被无限期挂起，覆盖大 JSON 序列化与流式传输的最坏情况。
		WriteTimeout: 30 * time.Second,
		// IdleTimeout 控制 keep-alive 连接复用窗口，平衡高并发与资源回收。
		IdleTimeout: 120 * time.Second,
		// MaxHeaderBytes 限制请求头大小，防止超大 Cookie/JWT 导致内存暴涨。
		MaxHeaderBytes: 1 << 21, // 2MB
	}
}

// Close 关停后台 goroutine
func (a *App) Close() {
	if a.cancel != nil {
		a.cancel()
	}
}

// @title AI Script API
// @version 1.0
// @description AI 短剧视频生成平台 REST API 文档
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 请输入 JWT token，格式: Bearer {token}

func newRouter(
	cfg *conf.Config,
	log *zap.Logger,
	jwtMgr *jwt.Manager,
	enforcer *casbin.Enforcer,
	h *handler.Handlers,
	store storage.Storage,
	db *gorm.DB,
	rdb *redis.Client,
) *gin.Engine {
	if cfg.App.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.RemoveExtraSlash = true

	// 安全: 限制 multipart 文件上传内存 (32MB)，超大文件写入临时目录
	r.MaxMultipartMemory = 32 << 20

	// 修复 P0 A5 — CORS 不再 AllowAllOrigins,改用可配置白名单
	corsCfg := cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Authorization",
			"X-API-Token",
			"Content-Type",
			"X-Trace-Id",
			"X-Request-Id",
		},
		ExposeHeaders: []string{"X-Request-Id", "X-Trace-Id"},
		MaxAge:       12 * time.Hour,
	}
	if len(cfg.App.Origins) > 0 {
		corsCfg.AllowOrigins = cfg.App.Origins
	} else if cfg.App.Env == "prod" {
		log.Warn("prod env but app.origins empty — CORS defaults to same-origin only")
		corsCfg.AllowOriginFunc = func(origin string) bool { return false }
	} else {
		corsCfg.AllowOriginFunc = func(origin string) bool { return true }
	}
	// Gzip 压缩：对 API 响应启用，排除 Prometheus / pprof / WebSocket / 健康检查 /
	// 上传路由（已压缩内容或流式传输，压缩反而增加 CPU 开销）。
	gzipMiddleware := gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{
		"/metrics",
		"/debug/pprof",
		"/ws/progress",
		"/healthz/live",
		"/healthz/ready",
		"/files/upload",
		"/uploads",
	}))

	// 修复 P0 — Recovery 必须在 middleware chain 最外层,防止 AccessLog/Validate 等 panic 导致进程崩溃
	r.Use(middleware.Recovery(log))

	// 修复 P0 — 探针与 metrics 必须在限流之前挂载,避免 K8s 探针被限流误杀
	// 修复 P0 B3 — /healthz 拆分为 liveness 与 readiness 探针
	r.GET("/healthz/live", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/healthz/ready", func(c *gin.Context) {
		healthy := true
		var errs []string
		if sqlDB, err := db.DB(); err != nil || sqlDB.Ping() != nil {
			healthy = false
			errs = append(errs, "db_ping_failed")
		}
		if rdb != nil {
			if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
				healthy = false
				errs = append(errs, "redis_ping_failed")
			}
		}
		if healthy {
			c.JSON(200, gin.H{"status": "ok"})
		} else {
			c.JSON(503, gin.H{"status": "unhealthy", "errors": errs})
		}
	})

	// 修复 P0 B4 — 暴露 Prometheus 兼容指标(runtime + 业务指标)
	r.GET("/metrics", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(200, metrics.FormatPrometheus())
	})

	// 修复 P0 — 404/405 统一返回 JSON 业务格式,避免 gin 默认 HTML 404
	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, errcode.ErrNotFound)
	})
	r.NoMethod(func(c *gin.Context) {
		response.Fail(c, errcode.ErrMethodNotAllowed)
	})

	r.Use(
		middleware.RequestID(),
		middleware.AccessLog(log),
		metricsMiddleware(),
		middleware.Validate(log),
		corsWithLog(corsCfg, log),
		gzipMiddleware,
		middleware.IPRateLimit(rdb, log),
		middleware.GlobalRateLimit(rdb, log),
		bodySizeLimitMiddleware(), // 非上传路由限制请求体 1MB，防 DoS
	)

	// 修复 P0 F1 — pprof 性能剖析端点(内网 IP 白名单保护,生产环境可用)
	prf := r.Group("/debug", func(c *gin.Context) {
		if isPrivateIP(c.ClientIP()) {
			c.Next()
			return
		}
		c.AbortWithStatus(403)
	})
	{
		prf.GET("/pprof/", gin.WrapF(pprof.Index))
		prf.GET("/pprof/cmdline", gin.WrapF(pprof.Cmdline))
		prf.GET("/pprof/profile", gin.WrapF(pprof.Profile))
		prf.GET("/pprof/symbol", gin.WrapF(pprof.Symbol))
		prf.GET("/pprof/trace", gin.WrapF(pprof.Trace))
		prf.GET("/pprof/allocs", gin.WrapH(pprof.Handler("allocs")))
		prf.GET("/pprof/block", gin.WrapH(pprof.Handler("block")))
		prf.GET("/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		prf.GET("/pprof/heap", gin.WrapH(pprof.Handler("heap")))
		prf.GET("/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
		prf.GET("/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
		prf.GET("/vars", gin.WrapF(expvar.Handler().ServeHTTP))
	}

	// 本地对象存储:暴露上传文件的静态访问 (需鉴权)
	// 修复 P0 #2 — 原 r.Static 在 JWT 之前 mount, /uploads/* 完全裸奔。
	// 改为 JWT 保护 + 路径白名单防穿越 (../ + 绝对路径 + 空/null 字节 + 软链接)。
	if prefix := storage.PublicPrefix(store); prefix != "" && prefix != "/" {
		if dir := storage.BaseDir(store); dir != "" {
			rootDir, _ := filepath.Abs(dir)
			filesGrp := r.Group(prefix, middleware.JWTAuth(jwtMgr))
			filesGrp.GET("/*filepath", func(c *gin.Context) {
				rel := strings.TrimPrefix(c.Param("filepath"), "/")
				// 加固:拒绝空路径、null 字节、路径穿越片段
				if rel == "" || strings.Contains(rel, "\x00") || strings.Contains(rel, "..") {
					response.Fail(c, errcode.ErrForbidden)
					return
				}
				full, err := filepath.Abs(filepath.Join(rootDir, rel))
				if err != nil || !strings.HasPrefix(full, rootDir+string(filepath.Separator)) && full != rootDir {
					response.Fail(c, errcode.ErrForbidden)
					return
				}
				// 加固:解析软链接,防止通过 symlink 绕过 rootDir
				if fi, err := os.Lstat(full); err == nil {
					if fi.Mode()&os.ModeSymlink != 0 {
						if realPath, err := filepath.EvalSymlinks(full); err == nil {
							if !strings.HasPrefix(realPath, rootDir+string(filepath.Separator)) && realPath != rootDir {
								response.Fail(c, errcode.ErrForbidden)
								return
							}
						}
					}
				}
				c.File(full)
			})
		}
	}

	// WebSocket 进度通道(从 query 取 token 校验)
	r.GET("/ws/progress", middleware.WSAuth(jwtMgr), h.WS.Progress)

	// Swagger 文档路由（仅非生产环境暴露）
	if cfg.App.Env != "prod" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		auth.POST("/login", h.Auth.Login)
		auth.POST("/logout", h.Auth.Logout)
		auth.POST("/refresh", h.Auth.Refresh)
	}

	// rbac 为闭包: 同时打 (object, action) 标签并执行 Casbin 校验
	// 修复 P0 #1 — 原 group 级 middleware.RBAC 在 route 级 rbac() 之前执行,
	// c.Get("rbac_obj")=nil 导致全员放行 (RBAC 名存实亡)。
	rbac := func(obj, act string) gin.HandlerFunc {
		return func(c *gin.Context) {
			roles, _ := c.Get("roles")
			rs, _ := roles.([]string)
			for _, role := range rs {
				// 超管直接放行
				if role == "super_admin" {
					c.Set("rbac_obj", obj)
					c.Set("rbac_act", act)
					c.Next()
					return
				}
				if ok, _ := enforcer.Enforce(role, obj, act); ok {
					c.Set("rbac_obj", obj)
					c.Set("rbac_act", act)
					c.Next()
					return
				}
			}
			response.Fail(c, errcode.ErrForbidden)
		}
	}

	authed := api.Group("",
		middleware.JWTAuth(jwtMgr),
	)
	{
		// 当前用户 ===========
		authed.GET("/users/me", h.User.Me)
		authed.POST("/users/me/password", h.User.ChangePassword)

		// 用户 ===========
		authed.GET("/users", rbac("user", "read"), h.User.List)
		authed.POST("/users", rbac("user", "create"), h.User.Create)
		authed.GET("/users/:id", rbac("user", "read"), h.User.Get)
		authed.PUT("/users/:id", rbac("user", "update"), h.User.Update)
		authed.DELETE("/users/:id", rbac("user", "delete"), h.User.Delete)
		authed.POST("/users/:id/reset_password", rbac("user", "update"), h.User.ResetPassword)

		// 部门 ===========
		authed.GET("/depts", rbac("dept", "read"), h.Dept.List)
		authed.POST("/depts", rbac("dept", "create"), h.Dept.Create)
		authed.GET("/depts/:id", rbac("dept", "read"), h.Dept.Get)
		authed.PUT("/depts/:id", rbac("dept", "update"), h.Dept.Update)
		authed.DELETE("/depts/:id", rbac("dept", "delete"), h.Dept.Delete)

		// 角色 ===========
		authed.GET("/roles", rbac("role", "read"), h.Role.List)
		authed.POST("/roles", rbac("role", "create"), h.Role.Create)
		authed.GET("/roles/:id", rbac("role", "read"), h.Role.Get)
		authed.PUT("/roles/:id", rbac("role", "update"), h.Role.Update)
		authed.DELETE("/roles/:id", rbac("role", "delete"), h.Role.Delete)
		authed.GET("/permissions", rbac("role", "read"), h.Role.ListPermissions)

		// 项目 ===========
		authed.GET("/projects", rbac("project", "read"), h.Project.List)
		authed.POST("/projects", rbac("project", "create"), h.Project.Create)
		authed.GET("/projects/:id", rbac("project", "read"), h.Project.Get)
		authed.PUT("/projects/:id", rbac("project", "update"), h.Project.Update)
		authed.DELETE("/projects/:id", rbac("project", "delete"), h.Project.Delete)
		authed.GET("/projects/:id/members", rbac("project", "read"), h.Project.ListMembers)
		authed.POST("/projects/:id/members", rbac("project", "update"), h.Project.AddMember)
		authed.DELETE("/projects/:id/members/:uid", rbac("project", "update"), h.Project.RemoveMember)

		// 模型 ===========
		authed.GET("/models", rbac("model", "read"), h.Model.List)
		authed.POST("/models", rbac("model", "create"), h.Model.Create)
		authed.GET("/models/:id", rbac("model", "read"), h.Model.Get)
		authed.PUT("/models/:id", rbac("model", "update"), h.Model.Update)
		authed.DELETE("/models/:id", rbac("model", "delete"), h.Model.Delete)
		authed.POST("/models/:id/healthcheck", rbac("model", "read"), h.Model.Healthcheck)

		// Sprint 2 - 剧本 / 分集 / 提示词 ===========
		authed.GET("/scripts", rbac("script", "read"), h.Script.List)
		authed.POST("/scripts", rbac("script", "create"), h.Script.Create)
		authed.GET("/scripts/:id", rbac("script", "read"), h.Script.Get)
		authed.DELETE("/scripts/:id", rbac("script", "delete"), h.Script.Delete)
		authed.GET("/scripts/:id/episodes", rbac("script", "read"), h.Script.ListEpisodes)
		// 修复 P0 A3 — AI 生成类接口按用户令牌桶限流(2/min, burst=5)
		authed.POST("/scripts/:id/split", rbac("script", "update"), middleware.RateLimit(rdb, log), middleware.Idempotency(rdb), h.Script.Split)

		authed.GET("/episodes/:id/prompts", rbac("prompt", "read"), h.Prompt.ListByEpisode)
		authed.GET("/episodes/:id/prompts/current", rbac("prompt", "read"), h.Prompt.GetCurrent)
		authed.POST("/episodes/:id/prompts/generate", rbac("prompt", "create"), middleware.Idempotency(rdb), h.Prompt.Generate)
		authed.POST("/prompts/:id/set_current", rbac("prompt", "update"), h.Prompt.SetCurrent)

		// Sprint 3 - 分镜 ===========
		authed.GET("/episodes/:id/storyboards", rbac("storyboard", "read"), h.Story.ListByEpisode)
		authed.POST("/episodes/:id/storyboards/generate", rbac("storyboard", "create"), middleware.Idempotency(rdb), h.Story.Generate)
		authed.POST("/episodes/:id/storyboards/bulk_save", rbac("storyboard", "update"), h.Story.BulkSave)
		authed.GET("/storyboards/:id", rbac("storyboard", "read"), h.Story.Get)
		authed.PUT("/storyboards/:id", rbac("storyboard", "update"), h.Story.Update)
		authed.DELETE("/storyboards/:id", rbac("storyboard", "delete"), h.Story.Delete)
		authed.POST("/storyboards/:id/apply_style", rbac("storyboard", "update"), h.Story.ApplyStyle)

		// Sprint 3 - 风格库 ===========
		authed.GET("/styles", rbac("style", "read"), h.Style.List)
		authed.POST("/styles", rbac("style", "create"), h.Style.Create)
		authed.GET("/styles/:id", rbac("style", "read"), h.Style.Get)
		authed.PUT("/styles/:id", rbac("style", "update"), h.Style.Update)
		authed.DELETE("/styles/:id", rbac("style", "delete"), h.Style.Delete)

		// Sprint 3 - 图片 ===========
		authed.GET("/images", rbac("image", "read"), h.Image.List)
		authed.GET("/images/:id", rbac("image", "read"), h.Image.Get)
		authed.POST("/images/generate", rbac("image", "create"), middleware.RateLimit(rdb, log), middleware.Idempotency(rdb), h.Image.Generate)
		authed.DELETE("/images/:id", rbac("image", "delete"), h.Image.Delete)

		// Sprint 3 - 短视频 ===========
		authed.GET("/short_videos", rbac("short_video", "read"), h.Short.List)
		authed.GET("/short_videos/:id", rbac("short_video", "read"), h.Short.Get)
		authed.POST("/short_videos/generate", rbac("short_video", "create"), middleware.RateLimit(rdb, log), middleware.Idempotency(rdb), h.Short.Generate)
		authed.DELETE("/short_videos/:id", rbac("short_video", "delete"), h.Short.Delete)

		// Sprint 3 - 上传 ===========
		authed.POST("/files/upload", rbac("upload", "create"), h.Upload.Upload)

		// Sprint 3 - 调用日志 ===========
		authed.GET("/invocations", rbac("invocation", "read"), h.Invocation.List)
		authed.GET("/invocations/stats", rbac("invocation", "read"), h.Invocation.Stats)

		// Sprint 4 - 完整视频 ===========
		authed.GET("/full_videos", rbac("full_video", "read"), h.Full.List)
		authed.POST("/full_videos", rbac("full_video", "create"), h.Full.Create)
		authed.GET("/full_videos/:id", rbac("full_video", "read"), h.Full.Get)
		authed.PUT("/full_videos/:id", rbac("full_video", "update"), h.Full.Update)
		authed.DELETE("/full_videos/:id", rbac("full_video", "delete"), h.Full.Delete)
		authed.POST("/full_videos/:id/render", rbac("full_video", "execute"), middleware.RateLimit(rdb, log), middleware.Idempotency(rdb), h.Full.Render)

		// Sprint 4 - 流水线 ===========
		authed.GET("/pipelines", rbac("pipeline", "read"), h.Pipeline.List)
		authed.POST("/pipelines", rbac("pipeline", "create"), h.Pipeline.Create)
		authed.GET("/pipelines/:id", rbac("pipeline", "read"), h.Pipeline.Get)
		authed.PUT("/pipelines/:id", rbac("pipeline", "update"), h.Pipeline.Update)
		authed.DELETE("/pipelines/:id", rbac("pipeline", "delete"), h.Pipeline.Delete)
		authed.POST("/pipelines/:id/run", rbac("pipeline", "execute"), middleware.RateLimit(rdb, log), middleware.Idempotency(rdb), h.Pipeline.Run)
		authed.GET("/pipelines/:id/runs", rbac("pipeline", "read"), h.Pipeline.ListRuns)
		authed.GET("/pipeline_runs/:id", rbac("pipeline", "read"), h.Pipeline.GetRun)
		authed.GET("/pipeline_runs/:id/steps", rbac("pipeline", "read"), h.Pipeline.ListSteps)

		// Sprint 5 - 审核 ===========
		authed.GET("/review/flows", rbac("review", "read"), h.Review.ListFlows)
		authed.GET("/review/flows/:id", rbac("review", "read"), h.Review.GetFlow)
		authed.GET("/review/flows/:id/nodes", rbac("review", "read"), h.Review.ListNodes)
		authed.POST("/review/records", rbac("review", "create"), h.Review.Submit)
		authed.GET("/review/records", rbac("review", "read"), h.Review.ListRecords)
		authed.GET("/review/records/:id", rbac("review", "read"), h.Review.GetRecord)
		authed.GET("/review/records/:id/actions", rbac("review", "read"), h.Review.ListActions)
		authed.POST("/review/records/:id/act", rbac("review", "update"), h.Review.Act)
		authed.POST("/review/records/:id/cancel", rbac("review", "update"), h.Review.Cancel)

		// Sprint 5 - 发布 ===========
		authed.GET("/publishes", rbac("publish", "read"), h.Publish.List)
		authed.POST("/publishes", rbac("publish", "create"), middleware.Idempotency(rdb), h.Publish.Publish)
		authed.GET("/publishes/:id", rbac("publish", "read"), h.Publish.Get)
		authed.POST("/publishes/:id/unpublish", rbac("publish", "update"), h.Publish.Unpublish)
		authed.POST("/publishes/:id/play", rbac("publish", "write"), h.Publish.IncPlay)
		authed.POST("/publishes/:id/download", rbac("publish", "write"), h.Publish.IncDownload)
		authed.PUT("/publishes/:id/watermark", rbac("publish", "update"), h.Publish.UpdateWatermark)

		// Sprint 5 - 计费 ===========
		authed.GET("/billing/quotas", rbac("billing", "read"), h.Billing.ListQuotas)
		authed.POST("/billing/quotas", rbac("billing", "create"), h.Billing.CreateQuota)
		authed.GET("/billing/quotas/:id", rbac("billing", "read"), h.Billing.GetQuota)
		authed.PUT("/billing/quotas/:id", rbac("billing", "update"), h.Billing.UpdateQuota)
		authed.DELETE("/billing/quotas/:id", rbac("billing", "delete"), h.Billing.DeleteQuota)
		authed.GET("/billing/daily", rbac("billing", "read"), h.Billing.ListDaily)

		// Sprint 5 - 审计 ===========
		authed.GET("/audit_logs", rbac("audit", "read"), h.Audit.List)

		// Sprint 5 - 灰度开关 ===========
		authed.GET("/feature_flags", rbac("feature_flag", "read"), h.FeatureFlag.List)
		authed.POST("/feature_flags", rbac("feature_flag", "create"), h.FeatureFlag.Create)
		authed.GET("/feature_flags/evaluate", h.FeatureFlag.Evaluate)
		authed.GET("/feature_flags/:id", rbac("feature_flag", "read"), h.FeatureFlag.Get)
		authed.PUT("/feature_flags/:id", rbac("feature_flag", "update"), h.FeatureFlag.Update)
		authed.DELETE("/feature_flags/:id", rbac("feature_flag", "delete"), h.FeatureFlag.Delete)
	}

	return r
}

// metricsMiddleware 收集 HTTP 请求数、耗时、业务错误数。
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		path := c.Request.URL.Path
		method := c.Request.Method

		metrics.RecordHTTP(method, path, status, duration)

		// 记录业务错误:如果 gin 写入了错误响应且是业务错误码
		if status >= 400 {
			// 尝试从 context 读取业务错误码
			if code, exists := c.Get("biz_error_code"); exists {
				if codeInt, ok := code.(int); ok {
					metrics.RecordBusinessError(codeInt)
				}
			}
		}
	}
}

// corsWithLog 包装 cors 中间件，当 CORS 拒绝（返回 403）时记录日志，便于排查跨域来源。
func corsWithLog(cfg cors.Config, log *zap.Logger) gin.HandlerFunc {
	handler := cors.New(cfg)
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		handler(c)
		if c.IsAborted() && c.Writer.Status() == http.StatusForbidden {
			log.Warn("cors rejected request",
				zap.String("rid", c.GetString("request_id")),
				zap.String("origin", origin),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method),
				zap.Strings("allow_origins", cfg.AllowOrigins),
			)
		}
	}
}

// bodySizeLimitMiddleware 对非文件上传路由限制请求体大小为 1MB，防止 DoS
func bodySizeLimitMiddleware() gin.HandlerFunc {
	const maxBodySize = 1 << 20 // 1MB
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 上传路由和 multipart 请求不限制（由 r.MaxMultipartMemory 控制）
		if c.Request.Method == "POST" && (strings.HasSuffix(path, "/files/upload") ||
			strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data")) {
			c.Next()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodySize)
		c.Next()
	}
}

// isPrivateIP 判断 IP 是否属于内网白名单：127.0.0.1/::1/10.0.0.0/8/172.16.0.0/12/192.168.0.0/16
func isPrivateIP(ipStr string) bool {
	if ipStr == "127.0.0.1" || ipStr == "::1" {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, cidr := range privateCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func newDB(cfg *conf.Config, log *zap.Logger) (*gorm.DB, error) {
	if cfg.MySQL.DSN == "" {
		return nil, fmt.Errorf("mysql dsn empty")
	}
	// 自定义 GORM Logger：仅记录慢查询（>=200ms），使用 zap 输出
	gl := newZapGormLogger(log, 200*time.Millisecond)
	db, err := gorm.Open(mysql.Open(cfg.MySQL.DSN), &gorm.Config{
		Logger: gl,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// 修复 P0 C5 — 连接池参数从配置读取,补充 ConnMaxIdleTime
	maxIdle := cfg.MySQL.MaxIdle
	if maxIdle <= 0 {
		maxIdle = 10
	}
	maxOpen := cfg.MySQL.MaxOpen
	if maxOpen <= 0 {
		maxOpen = 100
	}
	connMaxLifetime := time.Duration(cfg.MySQL.ConnMaxLifetime) * time.Second
	if connMaxLifetime <= 0 {
		connMaxLifetime = time.Hour
	}
	connMaxIdleTime := time.Duration(cfg.MySQL.ConnMaxIdleTime) * time.Second
	if connMaxIdleTime <= 0 {
		connMaxIdleTime = 30 * time.Minute
	}
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

// zapGormLogger 是一个使用 zap 的 gorm.Logger 实现，仅用于慢查询监控。
type zapGormLogger struct {
	log           *zap.Logger
	slowThreshold time.Duration
}

func newZapGormLogger(log *zap.Logger, slowThreshold time.Duration) gormlogger.Interface {
	return &zapGormLogger{
		log:           log,
		slowThreshold: slowThreshold,
	}
}

func (l *zapGormLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface { return l }
func (l *zapGormLogger) Info(context.Context, string, ...interface{})     {}
func (l *zapGormLogger) Warn(context.Context, string, ...interface{})     {}
func (l *zapGormLogger) Error(context.Context, string, ...interface{})    {}

func (l *zapGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	if elapsed < l.slowThreshold {
		return
	}
	sql, rowsAffected := fc()
	l.log.Warn("slow sql",
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rowsAffected),
		zap.String("sql", sql),
	)
}

// NewDB 是 newDB 的导出版本,供 Wire 与 worker 复用。
func NewDB(cfg *conf.Config, log *zap.Logger) (*gorm.DB, error) {
	return newDB(cfg, log)
}

// NewRedis 是 newRedis 的导出版本,供 Wire 与 worker 复用。
func NewRedis(cfg *conf.Config) *redis.Client {
	return newRedis(cfg)
}

func newRedis(cfg *conf.Config) *redis.Client {
	// 修复 P0 C5 — Redis 连接池参数从配置读取
	poolSize := cfg.Redis.PoolSize
	if poolSize <= 0 {
		poolSize = 20
	}
	minIdle := cfg.Redis.MinIdleConns
	if minIdle <= 0 {
		minIdle = 5
	}
	poolTimeout := time.Duration(cfg.Redis.PoolTimeout) * time.Second
	if poolTimeout <= 0 {
		poolTimeout = 5 * time.Second
	}
	return redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     poolSize,
		MinIdleConns: minIdle,
		PoolTimeout:  poolTimeout,
	})
}
