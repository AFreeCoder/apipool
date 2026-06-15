package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const reqLogHeaderBudget = 8 * 1024

type ReqLogCaptureService interface {
	GetCaptureState(ctx context.Context, userID int64, now time.Time) (*reqlog.CaptureState, bool)
	Submit(entry *reqlog.ReqLogEntry) bool
}

func ReqLogCaptureMiddleware(svc ReqLogCaptureService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil {
			c.Next()
			return
		}
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			c.Next()
			return
		}
		started := time.Now()
		state, ok := svc.GetCaptureState(c.Request.Context(), subject.UserID, started)
		if !ok || state == nil {
			c.Next()
			return
		}
		state.NormalizeTimes()
		if !state.ExpiresAt.IsZero() && started.After(state.ExpiresAt) {
			c.Next()
			return
		}
		reqlog.SetCaptureState(c, state)

		captureResp := shouldCaptureResponse(c.Request)
		originalWriter := c.Writer
		var writer *reqlog.CaptureWriter
		if captureResp {
			writer = reqlog.AcquireCaptureWriter(originalWriter, state.SingleResponseCap)
			c.Writer = writer
		}
		defer func() {
			if writer != nil {
				if c.Writer == writer {
					c.Writer = originalWriter
				}
				reqlog.ReleaseCaptureWriter(writer)
			}
		}()

		c.Next()

		var respBody []byte
		respTruncated := false
		if writer != nil {
			respBody = writer.CapturedCopy()
			respTruncated = writer.Truncated()
		}
		entry := buildReqLogEntry(c, state, started, respBody, respTruncated, captureResp)
		_ = svc.Submit(entry)
	}
}

func shouldCaptureResponse(r *http.Request) bool {
	if r == nil {
		return true
	}
	if isReqLogWebSocketUpgrade(r) {
		return false
	}
	if r.Method != http.MethodGet {
		return true
	}
	return !isReqLogMetadataGETPath(r.URL.Path)
}

func isReqLogWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Connection"))), "upgrade")
}

func isReqLogMetadataGETPath(path string) bool {
	p := strings.TrimRight(strings.TrimSpace(path), "/")
	switch p {
	case "/models",
		"/usage",
		"/v1/models",
		"/v1/usage",
		"/v1beta/models",
		"/antigravity/models",
		"/antigravity/v1/models",
		"/antigravity/v1/usage",
		"/antigravity/v1beta/models":
		return true
	}
	return strings.HasPrefix(p, "/v1beta/models/") ||
		strings.HasPrefix(p, "/antigravity/v1beta/models/")
}

func buildReqLogEntry(c *gin.Context, state *reqlog.CaptureState, started time.Time, respBody []byte, respTruncated bool, responseCaptured bool) *reqlog.ReqLogEntry {
	now := time.Now()
	reqSnap, _ := reqlog.RequestBodySnapshot(c)
	reqBody := []byte(nil)
	reqKind := reqlog.BodyKindNone
	reqTruncated := false
	if reqSnap != nil {
		reqBody = reqSnap.Body
		reqKind = reqSnap.Kind
		reqTruncated = reqSnap.Truncated
	}
	model, stream := extractModelStream(reqBody)
	if ctxModel, _ := c.Request.Context().Value(ctxkey.Model).(string); strings.TrimSpace(ctxModel) != "" {
		model = strings.TrimSpace(ctxModel)
	}
	isWS := isReqLogWebSocketUpgrade(c.Request)
	transport := "http"
	if isWS {
		transport = "ws"
	} else if stream || strings.Contains(strings.ToLower(c.Writer.Header().Get("Content-Type")), "text/event-stream") {
		transport = "sse"
	}
	if !responseCaptured {
		respBody = nil
		respTruncated = false
	}
	var accountID *int64
	if v, ok := c.Request.Context().Value(ctxkey.AccountID).(int64); ok && v > 0 {
		accountID = &v
	}
	platform, _ := c.Request.Context().Value(ctxkey.Platform).(string)
	if strings.TrimSpace(platform) == "" {
		if apiKey, ok := GetAPIKeyFromContext(c); ok && apiKey != nil && apiKey.Group != nil {
			platform = apiKey.Group.Platform
		}
	}
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	clientReqID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
	inbound, _ := c.Request.Context().Value(ctxkey.InboundEndpoint).(string)
	if inbound == "" && c.FullPath() != "" {
		inbound = c.FullPath()
	}
	status := http.StatusSwitchingProtocols
	respHeaders := map[string]string{}
	if !isWS {
		status = c.Writer.Status()
		if status == 0 {
			status = http.StatusOK
		}
		respHeaders = service.RedactHeaders(c.Writer.Header(), reqLogHeaderBudget)
	}
	errDetail := ""
	if v, ok := c.Get(service.OpsUpstreamErrorDetailKey); ok {
		errDetail, _ = v.(string)
	}
	return (&reqlog.ReqLogEntry{
		UserID:          state.UserID,
		SessionID:       state.SessionID,
		RequestID:       strings.TrimSpace(requestID),
		ClientReqID:     strings.TrimSpace(clientReqID),
		Timestamp:       started.UTC(),
		Method:          c.Request.Method,
		Path:            c.Request.URL.Path,
		InboundEndpoint: strings.TrimSpace(inbound),
		Model:           model,
		Stream:          stream,
		Transport:       transport,
		StatusCode:      status,
		DurationMs:      now.Sub(started).Milliseconds(),
		AccountID:       accountID,
		Platform:        strings.TrimSpace(platform),
		ClientIP:        c.ClientIP(),
		ReqHeaders:      service.RedactHeaders(c.Request.Header, reqLogHeaderBudget),
		RespHeaders:     respHeaders,
		ReqBody:         reqBody,
		ReqBodyKind:     reqKind,
		ReqTruncated:    reqTruncated,
		RespBody:        respBody,
		RespTruncated:   respTruncated,
		ErrorDetail:     errDetail,
	})
}

func extractModelStream(body []byte) (string, bool) {
	if len(body) == 0 || !json.Valid(body) {
		return "", false
	}
	var payload struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}
	return strings.TrimSpace(payload.Model), payload.Stream
}
