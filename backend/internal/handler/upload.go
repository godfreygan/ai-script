package handler

import (
	"io"
	"strings"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UploadHandler struct {
	upload service.UploadService
	log    *zap.Logger
}

func (h *UploadHandler) Upload(c *gin.Context) {
	ns := strings.TrimSpace(c.DefaultPostForm("namespace", "misc"))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	defer f.Close()
	res, err := h.upload.Save(c.Request.Context(), ns, fileHeader.Filename, fileHeader.Header.Get("Content-Type"), io.Reader(f), fileHeader.Size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}
