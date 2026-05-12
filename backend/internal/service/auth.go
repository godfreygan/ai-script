package service

import (
	"context"
	"errors"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/jwt"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	user *repo.UserRepo
	jwt  *jwt.Manager
	log  *zap.Logger
}

type LoginResult struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int         `json:"expires_in"`
	User         *model.User `json:"user"`
	Roles        []string    `json:"roles"`
}

const accessTokenExpiresIn = 7200

func (s *AuthService) Login(ctx context.Context, username, password, clientIP string) (*LoginResult, error) {
	if username == "" || password == "" {
		return nil, errcode.ErrParam.WithMsg("username or password empty")
	}
	u, err := s.user.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrUnauthorized.WithMsg("invalid username or password")
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if u.Status != 1 {
		return nil, errcode.ErrUnauthorized.WithMsg("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, errcode.ErrUnauthorized.WithMsg("invalid username or password")
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

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
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

func (s *AuthService) ChangePassword(ctx context.Context, uid int64, oldPw, newPw string) error {
	if uid <= 0 {
		return errcode.ErrParam
	}
	if len(newPw) < 6 {
		return errcode.ErrParam.WithMsg("password too short")
	}
	if oldPw == newPw {
		return errcode.ErrParam.WithMsg("new password must differ from old")
	}
	u, err := s.user.GetByID(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrUnauthorized
		}
		return errcode.ErrInternal.Wrap(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPw)); err != nil {
		return errcode.ErrUnauthorized.Wrap(errors.New("wrong password"))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		return errcode.ErrInternal.Wrap(err)
	}
	if err := s.user.UpdatePassword(ctx, uid, string(hash)); err != nil {
		return errcode.ErrInternal.Wrap(err)
	}
	return nil
}
