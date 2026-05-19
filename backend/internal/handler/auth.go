package handler

import (
	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthHandler struct {
	auth service.AuthService
	log  *zap.Logger
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	rid := c.GetString("request_id")
	clientIP := c.ClientIP()

	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("login bind failed",
			zap.String("rid", rid),
			zap.String("client_ip", clientIP),
			zap.Error(err),
		)
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}

	h.log.Info("login attempt",
		zap.String("rid", rid),
		zap.String("username", req.Username),
		zap.String("client_ip", clientIP),
	)

	r, err := h.auth.Login(c.Request.Context(), req.Username, req.Password, c.ClientIP())
	if err != nil {
		h.log.Warn("login failed",
			zap.String("rid", rid),
			zap.String("username", req.Username),
			zap.String("client_ip", clientIP),
			zap.Error(err),
		)
		response.Fail(c, err)
		return
	}

	h.log.Info("login success",
		zap.String("rid", rid),
		zap.String("username", req.Username),
		zap.Int64("uid", r.User.ID),
		zap.Strings("roles", r.Roles),
	)
	response.OK(c, r)
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	r, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, r)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if len(auth) >= 8 && auth[:7] == "Bearer " {
		_ = h.auth.Logout(c.Request.Context(), auth[7:])
	}
	response.OK(c, nil)
}
