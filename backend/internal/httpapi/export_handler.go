package httpapi

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	presentationexport "github.com/dasi0227/PPT-Agent/backend/internal/export"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ExportHandler struct{ svc *service.ExportService }

func NewExportHandler(svc *service.ExportService) *ExportHandler { return &ExportHandler{svc: svc} }

func (h *ExportHandler) Create(c *gin.Context) {
	var request struct {
		Format          presentationexport.Format `json:"format"`
		ClientRequestID string                    `json:"client_request_id"`
	}
	if c.ShouldBindJSON(&request) != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	operation, err := h.svc.Start(c.Request.Context(), c.Param("id"), request.Format, request.ClientRequestID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, exportView(operation.View()))
}
func (h *ExportHandler) Get(c *gin.Context) {
	operation, err := h.svc.Get(c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, exportView(operation.View()))
}
func exportView(view presentationexport.View) presentationexport.View {
	if view.Artifact != nil {
		view.Artifact.Download = "/api/v1/exports/" + view.ID + "/download"
	}
	return view
}
func (h *ExportHandler) Events(c *gin.Context) {
	after := parseLastEventID(c)
	writer, err := newSSEWriter(c)
	if err != nil {
		AbortWithError(c, ErrInternal("streaming unsupported"))
		return
	}
	events, stop, err := h.svc.Subscribe(c.Param("id"), after)
	if err != nil {
		h.handleError(c, err)
		return
	}
	defer stop()
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if writer.event(strconv.FormatInt(event.Seq, 10), event.Type, event.Payload) != nil {
				return
			}
		case <-ticker.C:
			if writer.heartbeat() != nil {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}
func (h *ExportHandler) Download(c *gin.Context) {
	manager := h.svc.Manager()
	operation, err := manager.BeginDelivery(c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	view := operation.View()
	artifact := view.Artifact
	if artifact == nil {
		manager.DeliveryFailed(operation)
		h.handleError(c, presentationexport.ErrNotReady)
		return
	}
	file, err := os.Open(artifact.Path)
	if err != nil {
		manager.DeliveryFailed(operation)
		AbortWithError(c, ErrInternal("artifact unavailable"))
		return
	}
	defer file.Close()
	c.Header("Content-Type", artifact.MIME)
	c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
	c.Header("Content-Disposition", contentDisposition(artifact.Name))
	c.Header("X-Content-Type-Options", "nosniff")
	if _, err = io.Copy(c.Writer, file); err != nil {
		manager.DeliveryFailed(operation)
		return
	}
	manager.Consume(operation)
}
func (h *ExportHandler) Cancel(c *gin.Context) {
	if err := h.svc.Cancel(c.Param("id")); err != nil && !errors.Is(err, presentationexport.ErrGone) {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func contentDisposition(name string) string {
	return `attachment; filename="download"; filename*=UTF-8''` + url.PathEscape(name)
}
func (h *ExportHandler) handleError(c *gin.Context, err error) {
	var snapshotErr *presentationexport.SnapshotError
	switch {
	case errors.As(err, &snapshotErr):
		details := map[string]any{}
		if len(snapshotErr.Missing) > 0 {
			details["slides"] = snapshotErr.Missing
		}
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: snapshotErr.Code, Message: snapshotErr.Message, Details: details})
	case errors.Is(err, service.ErrExportInvalid):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "EXPORT_INVALID", Message: "导出请求无效。"})
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "EXPORT_RUN_ACTIVE", Message: "Agent 任务运行中，暂时不能导出。"})
	case errors.Is(err, service.ErrGitCommitActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "EXPORT_GIT_COMMIT_ACTIVE", Message: "项目正在提交，暂时不能导出。"})
	case errors.Is(err, presentationexport.ErrAlreadyActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "EXPORT_ALREADY_ACTIVE", Message: "当前项目已有导出任务。"})
	case errors.Is(err, presentationexport.ErrDownloadInProgress):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "EXPORT_DOWNLOAD_IN_PROGRESS", Message: "文件正在下载。"})
	case errors.Is(err, presentationexport.ErrNotReady):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "EXPORT_NOT_READY", Message: "导出文件尚未就绪。"})
	case errors.Is(err, presentationexport.ErrGone):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusGone, Code: "EXPORT_CONSUMED", Message: "导出文件已失效，请重新导出。"})
	case errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("export or project not found"))
	default:
		AbortWithError(c, ErrInternal("export failed"))
	}
}
