package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/conf"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/handler"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/middleware"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	pkgcasbin "git.myscrm.cn/ganqx01/ai-script/backend/pkg/casbin"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/crypto"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/jwt"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/queue"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/casbin/casbin/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
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

func NewApp(cfg *conf.Config, log *zap.Logger) (*App, error) {
	db, err := newDB(cfg)
	if err != nil {
		return nil, err
	}
	rdb := newRedis(cfg)
	enforcer, err := pkgcasbin.New(db, "./configs/rbac_model.conf")
	if err != nil {
		return nil, err
	}

	jwtMgr := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.AccessExpiresIn, cfg.JWT.RefreshExpiresIn)
	cipher, err := crypto.New(cfg.Crypto.Key)
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}
	taskClient := queue.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	registry := adapter.NewRegistry()

	store, err := storage.New(storage.FactoryConfig{
		Provider:   cfg.Storage.Provider,
		Endpoint:   cfg.Storage.Endpoint,
		Region:     cfg.Storage.Region,
		Bucket:     cfg.Storage.Bucket,
		AccessKey:  cfg.Storage.AccessKey,
		SecretKey:  cfg.Storage.SecretKey,
		PublicHost: cfg.Storage.PublicHost,
	})
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}

	hub := ws.NewHub(log)
	hub.BindRedis(rdb, "")
	hubCtx, cancel := context.WithCancel(context.Background())
	go hub.Run(hubCtx)

	repos := repo.NewRepositories(db, rdb)
	// AutoMigrate 所有业务表(只增不删,MVP 阶段安全)
	migCtx, mcancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := repos.Migrate(migCtx); err != nil {
		log.Warn("auto-migrate failed", zap.Error(err))
	}
	mcancel()
	services := service.NewServices(repos, jwtMgr, enforcer, cipher, registry, taskClient, hub, store, log)
	handlers := handler.NewHandlers(services, hub, log)

	// 启动时同步 Casbin 策略 + 加载所有 enabled 模型 adapter
	ctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ccancel()
	if err := services.Role.SyncCasbin(ctx); err != nil {
		log.Warn("sync casbin failed", zap.Error(err))
	}
	if err := services.Model.LoadAllAdapters(ctx); err != nil {
		log.Warn("load adapters failed", zap.Error(err))
	}

	app := &App{cfg: cfg, log: log, db: db, rdb: rdb, enforcer: enforcer, services: services, hub: hub, store: store, cancel: cancel}
	app.router = newRouter(cfg, log, jwtMgr, enforcer, handlers, store)
	return app, nil
}

func (a *App) Router() *gin.Engine { return a.router }

// Close 关停后台 goroutine
func (a *App) Close() {
	if a.cancel != nil {
		a.cancel()
	}
}

func newRouter(
	cfg *conf.Config,
	log *zap.Logger,
	jwtMgr *jwt.Manager,
	enforcer *casbin.Enforcer,
	h *handler.Handlers,
	store storage.Storage,
) *gin.Engine {
	if cfg.App.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.AccessLog(log),
		gin.Recovery(),
		cors.New(cors.Config{
			AllowAllOrigins: true,
			AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:    []string{"Authorization", "X-API-Token", "Content-Type"},
			MaxAge:          12 * time.Hour,
		}),
	)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	// 本地对象存储:暴露上传文件的静态访问 (需鉴权)
	// 修复 P0 #2 — 原 r.Static 在 JWT 之前 mount, /uploads/* 完全裸奔。
	// 改为 JWT 保护 + 路径白名单防穿越 (../ + 绝对路径)。
	if prefix := storage.PublicPrefix(store); prefix != "" && prefix != "/" {
		if dir := storage.BaseDir(store); dir != "" {
			rootDir, _ := filepath.Abs(dir)
			filesGrp := r.Group(prefix, middleware.JWTAuth(jwtMgr))
			filesGrp.GET("/*filepath", func(c *gin.Context) {
				rel := strings.TrimPrefix(c.Param("filepath"), "/")
				full, err := filepath.Abs(filepath.Join(rootDir, rel))
				if err != nil || !strings.HasPrefix(full, rootDir) {
					response.Fail(c, errcode.ErrForbidden)
					return
				}
				c.File(full)
			})
		}
	}

	// WebSocket 进度通道(从 query 取 token 校验)
	r.GET("/ws/progress", middleware.WSAuth(jwtMgr), h.WS.Progress)

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
		authed.POST("/scripts/:id/split", rbac("script", "update"), h.Script.Split)

		authed.GET("/episodes/:id/prompts", rbac("prompt", "read"), h.Prompt.ListByEpisode)
		authed.GET("/episodes/:id/prompts/current", rbac("prompt", "read"), h.Prompt.GetCurrent)
		authed.POST("/episodes/:id/prompts/generate", rbac("prompt", "create"), h.Prompt.Generate)
		authed.POST("/prompts/:id/set_current", rbac("prompt", "update"), h.Prompt.SetCurrent)

		// Sprint 3 - 分镜 ===========
		authed.GET("/episodes/:id/storyboards", rbac("storyboard", "read"), h.Story.ListByEpisode)
		authed.POST("/episodes/:id/storyboards/generate", rbac("storyboard", "create"), h.Story.Generate)
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
		authed.POST("/images/generate", rbac("image", "create"), h.Image.Generate)
		authed.DELETE("/images/:id", rbac("image", "delete"), h.Image.Delete)

		// Sprint 3 - 短视频 ===========
		authed.GET("/short_videos", rbac("short_video", "read"), h.Short.List)
		authed.GET("/short_videos/:id", rbac("short_video", "read"), h.Short.Get)
		authed.POST("/short_videos/generate", rbac("short_video", "create"), h.Short.Generate)
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
		authed.POST("/full_videos/:id/render", rbac("full_video", "execute"), h.Full.Render)

		// Sprint 4 - 流水线 ===========
		authed.GET("/pipelines", rbac("pipeline", "read"), h.Pipeline.List)
		authed.POST("/pipelines", rbac("pipeline", "create"), h.Pipeline.Create)
		authed.GET("/pipelines/:id", rbac("pipeline", "read"), h.Pipeline.Get)
		authed.PUT("/pipelines/:id", rbac("pipeline", "update"), h.Pipeline.Update)
		authed.DELETE("/pipelines/:id", rbac("pipeline", "delete"), h.Pipeline.Delete)
		authed.POST("/pipelines/:id/run", rbac("pipeline", "execute"), h.Pipeline.Run)
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
		authed.POST("/publishes", rbac("publish", "create"), h.Publish.Publish)
		authed.GET("/publishes/:id", rbac("publish", "read"), h.Publish.Get)
		authed.POST("/publishes/:id/unpublish", rbac("publish", "update"), h.Publish.Unpublish)
		authed.POST("/publishes/:id/play", rbac("publish", "read"), h.Publish.IncPlay)
		authed.POST("/publishes/:id/download", rbac("publish", "read"), h.Publish.IncDownload)
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

func newDB(cfg *conf.Config) (*gorm.DB, error) {
	if cfg.MySQL.DSN == "" {
		return nil, fmt.Errorf("mysql dsn empty")
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQL.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func newRedis(cfg *conf.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
}
