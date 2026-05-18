package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/jwt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 登录失败锁定常量
const (
	loginFailMax        = 5
	loginFailWindow     = 5 * time.Minute
	loginLockDuration   = 30 * time.Minute
	loginAttemptsPrefix = "login_attempts"
	loginLockedPrefix   = "login_locked"
)

type authService struct {
	user *repo.UserRepo
	jwt  *jwt.Manager
	log  *zap.Logger
	rdb  *redis.Client
}

type LoginResult struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int         `json:"expires_in"`
	User         *model.User `json:"user"`
	Roles        []string    `json:"roles"`
}

const accessTokenExpiresIn = 7200

func (s *authService) loginAttemptsKey(username string) string {
	return fmt.Sprintf("%s:%s", loginAttemptsPrefix, username)
}

func (s *authService) loginLockedKey(username string) string {
	return fmt.Sprintf("%s:%s", loginLockedPrefix, username)
}

func (s *authService) Login(ctx context.Context, username, password, clientIP string) (*LoginResult, error) {
	if username == "" || password == "" {
		return nil, errcode.ErrParam.WithMsg("username or password empty")
	}

	// 检查账户是否被锁定
	if s.rdb != nil {
		locked, err := s.rdb.Exists(ctx, s.loginLockedKey(username)).Result()
		if err == nil && locked > 0 {
			return nil, errcode.ErrAccountLocked.WithMsg("登录尝试过多，请 30 分钟后再试")
		}
	}

	u, err := s.user.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.recordLoginFailure(ctx, username)
			return nil, errcode.ErrUnauthorized.WithMsg("invalid username or password")
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if u.Status != 1 {
		return nil, errcode.ErrUnauthorized.WithMsg("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		s.recordLoginFailure(ctx, username)
		return nil, errcode.ErrUnauthorized.WithMsg("invalid username or password")
	}

	// 登录成功，清除失败计数
	if s.rdb != nil {
		if delErr := s.rdb.Del(ctx, s.loginAttemptsKey(username)).Err(); delErr != nil {
			s.log.Warn("clear login attempts failed", zap.String("username", username), zap.Error(delErr))
		}
	}

	// 检测存量用户密码是否符合当前复杂度策略，仅记录日志，不阻止登录
	if IsWeakPassword(password, u.Username) {
		s.log.Warn("user password does not meet current complexity policy",
			zap.Int64("uid", u.ID),
			zap.String("username", u.Username),
			zap.String("client_ip", clientIP),
		)
	}

	roles, err := s.user.GetRoleCodes(ctx, u.ID)
	if err != nil {
		s.log.Warn("load roles failed", zap.Int64("uid", u.ID), zap.Error(err))
	}
	if len(roles) == 0 {
		roles = []string{"viewer"}
	}

	access, refresh, err := s.jwt.Issue(&jwt.Claims{
		UserID:   u.ID,
		Username: u.Username,
		DeptID:   u.DeptID,
		Roles:    roles,
	})
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}

	if clientIP != "" {
		if err := s.user.UpdateLastLogin(ctx, u.ID, clientIP); err != nil {
			s.log.Warn("update last login failed", zap.Int64("uid", u.ID), zap.Error(err))
		}
	}
	now := time.Now()
	u.LastLoginAt = &now
	u.PasswordHash = "" // 不外泄

	return &LoginResult{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    accessTokenExpiresIn,
		User:         u,
		Roles:        roles,
	}, nil
}

// recordLoginFailure 记录登录失败并检查是否触发锁定
func (s *authService) recordLoginFailure(ctx context.Context, username string) {
	if s.rdb == nil {
		return
	}
	key := s.loginAttemptsKey(username)
	pipe := s.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, loginFailWindow)
	res, err := pipe.Exec(ctx)
	if err != nil {
		s.log.Warn("record login failure failed", zap.String("username", username), zap.Error(err))
		return
	}
	if len(res) > 0 {
		if cnt, ok := res[0].(*redis.IntCmd); ok && cnt.Val() >= loginFailMax {
			if err := s.rdb.Set(ctx, s.loginLockedKey(username), "1", loginLockDuration).Err(); err != nil {
				s.log.Warn("set login lock failed", zap.String("username", username), zap.Error(err))
			}
		}
	}
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	if refreshToken == "" {
		return nil, errcode.ErrParam.WithMsg("refresh_token empty")
	}
	claims, err := s.jwt.Parse(refreshToken)
	if err != nil {
		return nil, errcode.ErrTokenInvalid.Wrap(err)
	}
	// 必须是 refresh token,access token 不允许用来续签
	if claims.Subject != "refresh" {
		return nil, errcode.ErrTokenInvalid.WithMsg("not a refresh token")
	}
	u, err := s.user.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrUnauthorized
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if u.Status != 1 {
		return nil, errcode.ErrUnauthorized.WithMsg("user disabled")
	}
	roles, err := s.user.GetRoleCodes(ctx, u.ID)
	if err != nil {
		s.log.Warn("load roles failed", zap.Int64("uid", u.ID), zap.Error(err))
	}
	if len(roles) == 0 {
		roles = []string{"viewer"}
	}
	access, newRefresh, err := s.jwt.Issue(&jwt.Claims{
		UserID:   u.ID,
		Username: u.Username,
		DeptID:   u.DeptID,
		Roles:    roles,
	})
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	u.PasswordHash = ""
	return &LoginResult{
		AccessToken:  access,
		RefreshToken: newRefresh,
		ExpiresIn:    accessTokenExpiresIn,
		User:         u,
		Roles:        roles,
	}, nil
}

func (s *authService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return errcode.ErrParam.WithMsg("token empty")
	}
	claims, err := s.jwt.Parse(token)
	if err != nil {
		// 即使 token 已过期或无效，也视为 logout 成功（幂等）
		return nil
	}
	if s.rdb == nil || claims.ID == "" {
		return nil
	}
	// 计算剩余有效期作为黑名单 TTL
	var ttl time.Duration
	if claims.ExpiresAt != nil {
		remaining := time.Until(claims.ExpiresAt.Time)
		if remaining > 0 {
			ttl = remaining
		}
	}
	key := fmt.Sprintf("token:blacklist:%s", claims.ID)
	if err := s.rdb.Set(ctx, key, "1", ttl).Err(); err != nil {
		s.log.Warn("add token to blacklist failed", zap.String("jti", claims.ID), zap.Error(err))
		// 不阻断 logout 流程，但记录日志
	}
	return nil
}

func (s *authService) ChangePassword(ctx context.Context, uid int64, oldPw, newPw string) error {
	if uid <= 0 {
		return errcode.ErrParam
	}
	u, err := s.user.GetByID(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrUnauthorized
		}
		return errcode.ErrInternal.Wrap(err)
	}
	if err := ValidatePassword(newPw, u.Username); err != nil {
		return err
	}
	if oldPw == newPw {
		return errcode.ErrParam.WithMsg("new password must differ from old")
	}
	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPw)); bcryptErr != nil {
		return errcode.ErrUnauthorized.Wrap(errors.New("wrong password"))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), 12)
	if err != nil {
		return errcode.ErrInternal.Wrap(err)
	}
	if err := s.user.UpdatePassword(ctx, uid, string(hash)); err != nil {
		return errcode.ErrInternal.Wrap(err)
	}
	return nil
}
