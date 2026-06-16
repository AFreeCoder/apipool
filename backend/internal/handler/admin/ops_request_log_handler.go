package admin

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type enableRequestLogRequest struct {
	TTLSeconds int64  `json:"ttl_seconds"`
	Reason     string `json:"reason"`
	Force      bool   `json:"force"`
}

func (h *OpsHandler) EnableRequestLog(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	userID, ok := parsePositiveInt64Param(c, "user_id", "Invalid user_id")
	if !ok {
		return
	}
	var req enableRequestLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	adminID := currentAdminID(c)
	state, memory, err := h.reqLogService.Enable(c.Request.Context(), service.ReqLogEnableInput{
		UserID:  userID,
		AdminID: adminID,
		TTL:     time.Duration(req.TTLSeconds) * time.Second,
		Reason:  req.Reason,
		Force:   req.Force,
	})
	if err != nil {
		writeReqLogError(c, err)
		return
	}
	response.Success(c, gin.H{"session": state, "memory": memory})
}

func (h *OpsHandler) DisableRequestLog(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	userID, ok := parsePositiveInt64Param(c, "user_id", "Invalid user_id")
	if !ok {
		return
	}
	if err := h.reqLogService.Disable(c.Request.Context(), userID, currentAdminID(c)); err != nil {
		writeReqLogError(c, err)
		return
	}
	response.Success(c, gin.H{"disabled": true})
}

func (h *OpsHandler) GetRequestLogStatus(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	userID, ok := parsePositiveInt64Param(c, "user_id", "Invalid user_id")
	if !ok {
		return
	}
	status, err := h.reqLogService.Status(c.Request.Context(), userID)
	if err != nil {
		writeReqLogError(c, err)
		return
	}
	response.Success(c, status)
}

func (h *OpsHandler) ListActiveRequestLogs(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	items, err := h.reqLogService.ListActive(c.Request.Context())
	if err != nil {
		writeReqLogError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "count": len(items)})
}

func (h *OpsHandler) ListRequestLogSessions(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	userID, ok := parsePositiveInt64Param(c, "user_id", "Invalid user_id")
	if !ok {
		return
	}
	limit := parseBoundedInt(c.Query("limit"), 50, 1, 100)
	items, err := h.reqLogService.ListSessions(c.Request.Context(), currentAdminID(c), userID, limit)
	if err != nil {
		writeReqLogError(c, err)
		return
	}
	response.Success(c, items)
}

func (h *OpsHandler) ListRequestLogItems(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	page, pageSize := response.ParsePagination(c)
	if pageSize > 500 {
		pageSize = 500
	}
	items, total, _, err := h.reqLogService.ListItems(c.Request.Context(), currentAdminID(c), sessionID, page, pageSize)
	if err != nil {
		writeReqLogError(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *OpsHandler) GetRequestLogItem(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	seq, ok := parsePositiveInt64Param(c, "seq", "Invalid seq")
	if !ok {
		return
	}
	item, _, err := h.reqLogService.GetItem(c.Request.Context(), currentAdminID(c), sessionID, seq)
	if err != nil {
		writeReqLogError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *OpsHandler) CreateRequestLogDownloadToken(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	token, expiresAt, err := h.reqLogService.CreateDownloadToken(c.Request.Context(), currentAdminID(c), sessionID)
	if err != nil {
		writeReqLogError(c, err)
		return
	}
	url := fmt.Sprintf("/api/v1/ops-download/request-logs/sessions/%s/download?token=%s", sessionID, token)
	response.Success(c, gin.H{"url": url, "expires_at": expiresAt})
}

func (h *OpsHandler) DownloadRequestLogSession(c *gin.Context) {
	if h.reqLogService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Request log service not available")
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	adminID, ok := h.authenticateReqLogDownload(c, sessionID)
	if !ok {
		return
	}
	if !h.requireDownloadCompliance(c, adminID) {
		return
	}
	stats, userID, err := h.reqLogService.GetSessionStats(c.Request.Context(), sessionID)
	if err != nil {
		writeReqLogError(c, err)
		return
	}

	var bw *bufio.Writer
	started := false
	startStream := func() error {
		if started {
			return nil
		}
		c.Header("Content-Type", "application/x-ndjson")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="request-log-%s.ndjson"`, sanitizeFilenamePart(sessionID)))
		c.Status(http.StatusOK)
		bw = bufio.NewWriter(c.Writer)
		started = true
		// P7：首行 metadata 补齐设计 §3.8 要求的字段，便于解析方校验上下文与完整性。
		meta := map[string]any{
			"schema_version": 1,
			"session_id":     sessionID,
			"user_id":        userID,
		}
		if stats != nil {
			meta["started_at"] = stats.StartedAt
			meta["expires_at"] = stats.ExpiresAt
			meta["item_count"] = stats.ItemCount
			meta["truncated"] = stats.Truncated
			meta["dropped_count"] = stats.DroppedCount
		}
		raw, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		if _, err := bw.Write(raw); err != nil {
			return err
		}
		_, err = bw.WriteString("\n")
		return err
	}

	_, err = h.reqLogService.DownloadItems(c.Request.Context(), adminID, sessionID, func(item *reqlog.ReqLogEntry) error {
		if err := startStream(); err != nil {
			return err
		}
		raw, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err := bw.Write(raw); err != nil {
			return err
		}
		_, err = bw.WriteString("\n")
		return err
	})
	if err != nil {
		if bw != nil {
			_ = bw.Flush()
		}
		if !started {
			writeReqLogError(c, err)
		}
		return
	}
	if !started {
		if err := startStream(); err != nil {
			return
		}
	}
	_ = bw.Flush()
}

func (h *OpsHandler) requireDownloadCompliance(c *gin.Context, adminID int64) bool {
	if h.settingService == nil {
		return true
	}
	if adminID <= 0 {
		response.Unauthorized(c, "Authorization required")
		return false
	}
	acknowledged, err := h.settingService.IsAdminComplianceAcknowledged(c.Request.Context(), adminID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error")
		return false
	}
	if acknowledged {
		return true
	}
	c.JSON(http.StatusLocked, gin.H{
		"code":    "ADMIN_COMPLIANCE_ACK_REQUIRED",
		"message": "administrator compliance acknowledgement is required",
		"metadata": gin.H{
			"version":          service.AdminComplianceVersion,
			"document_path_zh": service.AdminComplianceDocumentPathZH,
			"document_path_en": service.AdminComplianceDocumentPathEN,
			"document_url_zh":  service.AdminComplianceDocumentURLZH,
			"document_url_en":  service.AdminComplianceDocumentURLEN,
		},
	})
	c.Abort()
	return false
}

func (h *OpsHandler) authenticateReqLogDownload(c *gin.Context, sessionID string) (int64, bool) {
	if token := strings.TrimSpace(c.Query("token")); token != "" {
		adminID, err := h.reqLogService.ConsumeDownloadToken(c.Request.Context(), token, sessionID)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired download token")
			return 0, false
		}
		return adminID, true
	}
	if h.downloadAdminAuth == nil {
		response.Unauthorized(c, "Authorization required")
		return 0, false
	}
	h.downloadAdminAuth(c)
	if c.IsAborted() {
		return 0, false
	}
	adminID := currentAdminID(c)
	if adminID <= 0 {
		response.Unauthorized(c, "Authorization required")
		return 0, false
	}
	return adminID, true
}

func currentAdminID(c *gin.Context) int64 {
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		return subject.UserID
	}
	return 0
}

func parsePositiveInt64Param(c *gin.Context, name, message string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, message)
		return 0, false
	}
	return id, true
}

func parseBoundedInt(raw string, fallback, min, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func writeReqLogError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrReqLogDisabled):
		response.Error(c, http.StatusServiceUnavailable, "Request log is disabled")
	case errors.Is(err, service.ErrReqLogReasonRequired):
		response.BadRequest(c, "reason is required")
	case errors.Is(err, service.ErrReqLogAlreadyEnabled):
		response.Error(c, http.StatusConflict, "Request log is already enabled")
	case errors.Is(err, service.ErrReqLogConcurrentLimit):
		response.Error(c, http.StatusTooManyRequests, "Request log concurrent session limit reached")
	case errors.Is(err, service.ErrReqLogNotFound):
		response.NotFound(c, "Request log session not found")
	case errors.Is(err, service.ErrReqLogUnauthorized):
		response.Unauthorized(c, "Unauthorized")
	default:
		response.Error(c, http.StatusInternalServerError, err.Error())
	}
}

func sanitizeFilenamePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "session"
	}
	return b.String()
}
