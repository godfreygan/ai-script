package response

import (
	"errors"
	"net/http"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id,omitempty"`
}

type Page struct {
	List     interface{} `json:"list"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int64       `json:"total"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{
		Code: 0, Message: "ok", Data: data,
		RequestID: c.GetString("request_id"),
	})
}

func Fail(c *gin.Context, err error) {
	var e *errcode.Error
	if !errors.As(err, &e) {
		e = errcode.ErrInternal.Wrap(err)
	}
	httpCode := mapHTTP(e.Code)
	c.AbortWithStatusJSON(httpCode, Envelope{
		Code: e.Code, Message: e.Message,
		RequestID: c.GetString("request_id"),
	})
}

func mapHTTP(code int) int {
	switch {
	case code == 0:
		return http.StatusOK
	case code >= 40000 && code < 40100:
		return http.StatusBadRequest
	case code >= 40100 && code < 40200:
		return http.StatusUnauthorized
	case code >= 40300 && code < 40400:
		return http.StatusForbidden
	case code == 40400:
		return http.StatusNotFound
	case code >= 40900 && code < 41000:
		return http.StatusConflict
	case code == 40290 || code == 40291:
		return http.StatusPaymentRequired
	case code == 42900:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
