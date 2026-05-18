package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Validate 是一个轻量的全局请求参数校验中间件。
// 对常见参数进行前置校验，避免非法参数进入业务层。
func Validate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 路径参数 id 必须是正整数
		if idStr := c.Param("id"); idStr != "" {
			if id, err := strconv.ParseInt(idStr, 10, 64); err != nil || id <= 0 {
				response.Fail(c, errcode.ErrParam.WithMsg("invalid path param id"))
				c.Abort()
				return
			}
		}

		// 2. 路径参数 uid 必须是正整数
		if uidStr := c.Param("uid"); uidStr != "" {
			if uid, err := strconv.ParseInt(uidStr, 10, 64); err != nil || uid <= 0 {
				response.Fail(c, errcode.ErrParam.WithMsg("invalid path param uid"))
				c.Abort()
				return
			}
		}

		// 3. 分页参数 page/size 范围校验
		pageStr := c.Query("page")
		sizeStr := c.Query("size")
		if pageStr != "" {
			if page, err := strconv.Atoi(pageStr); err != nil || page < 1 {
				response.Fail(c, errcode.ErrParam.WithMsg("invalid query param page"))
				c.Abort()
				return
			}
		}
		if sizeStr != "" {
			if size, err := strconv.Atoi(sizeStr); err != nil || size < 1 || size > 1000 {
				response.Fail(c, errcode.ErrParam.WithMsg("invalid query param size"))
				c.Abort()
				return
			}
		}

		// 4. Content-Type 校验：POST/PUT/PATCH 非 multipart 请求应为 application/json
		method := c.Request.Method
		if (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) &&
			c.Request.Body != nil &&
			c.Request.ContentLength > 0 &&
			!strings.Contains(c.ContentType(), "multipart/form-data") {
			if !strings.Contains(c.ContentType(), "application/json") {
				response.Fail(c, errcode.ErrParam.WithMsg("Content-Type must be application/json"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// ValidateQuery 校验 query 参数并绑定到结构体。
// 校验失败时统一返回 400 + response.Fail(c, errcode.ErrParam)。
func ValidateQuery(schema interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.ShouldBindQuery(schema); err != nil {
			response.Fail(c, errcode.ErrParam.Wrap(err))
			c.Abort()
			return
		}
		c.Set("validated_query", schema)
		c.Next()
	}
}

// ValidateJSON 校验 JSON body 并绑定到结构体。
// 校验失败时统一返回 400 + response.Fail(c, errcode.ErrParam)。
func ValidateJSON(schema interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.ShouldBindJSON(schema); err != nil {
			response.Fail(c, errcode.ErrParam.Wrap(err))
			c.Abort()
			return
		}
		c.Set("validated_body", schema)
		c.Next()
	}
}

// ValidationErrors 把 validator.ValidationErrors 转换为可读字符串。
func ValidationErrors(err error) string {
	if ve, ok := err.(validator.ValidationErrors); ok {
		msgs := make([]string, 0, len(ve))
		for _, fe := range ve {
			msgs = append(msgs, fe.Field()+":"+fe.Tag())
		}
		return "validation failed: " + joinStrings(msgs, ", ")
	}
	return err.Error()
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	if len(ss) == 1 {
		return ss[0]
	}
	var b []byte
	for i, s := range ss {
		if i > 0 {
			b = append(b, sep...)
		}
		b = append(b, s...)
	}
	return string(b)
}

// BindAndValidateJSON 在 handler 内部直接调用，用于一次性绑定+校验 JSON body。
// 成功返回 true 并已写入 c；失败返回 false，已自动发送 400 响应。
func BindAndValidateJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return false
	}
	return true
}

// BindAndValidateQuery 在 handler 内部直接调用，用于一次性绑定+校验 query 参数。
func BindAndValidateQuery(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return false
	}
	return true
}
