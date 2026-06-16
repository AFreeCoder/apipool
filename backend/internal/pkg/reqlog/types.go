package reqlog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	BodyKindNone   = "none"
	BodyKindText   = "text"
	BodyKindBinary = "binary"

	OverflowDropOldest = "drop_oldest"
	OverflowStop       = "stop"
)

const (
	captureStateKey         = "_reqlog_capture_state"
	requestBodySnapshotKey  = "_reqlog_request_body_snapshot"
	defaultBinaryPreviewLen = 64
)

type CaptureState struct {
	UserID             int64         `json:"user_id"`
	SessionID          string        `json:"session_id"`
	StartedAt          time.Time     `json:"-"`
	ExpiresAt          time.Time     `json:"-"`
	StartedAtUnix      int64         `json:"started_at"`
	ExpiresAtUnix      int64         `json:"expires_at"`
	StartedByAdminID   int64         `json:"started_by_admin_id"`
	MaxBytes           int64         `json:"max_bytes"`
	MaxItems           int           `json:"max_items"`
	SingleRequestCap   int           `json:"single_request_cap"`
	SingleResponseCap  int           `json:"single_response_cap"`
	OverflowStrategy   string        `json:"overflow_strategy"`
	Reason             string        `json:"reason"`
	RetentionAfterStop time.Duration `json:"-"`
}

func (s *CaptureState) NormalizeTimes() {
	if s == nil {
		return
	}
	if s.StartedAt.IsZero() && s.StartedAtUnix > 0 {
		s.StartedAt = time.Unix(s.StartedAtUnix, 0).UTC()
	}
	if s.ExpiresAt.IsZero() && s.ExpiresAtUnix > 0 {
		s.ExpiresAt = time.Unix(s.ExpiresAtUnix, 0).UTC()
	}
	if !s.StartedAt.IsZero() {
		s.StartedAtUnix = s.StartedAt.Unix()
	}
	if !s.ExpiresAt.IsZero() {
		s.ExpiresAtUnix = s.ExpiresAt.Unix()
	}
	if s.OverflowStrategy == "" {
		s.OverflowStrategy = OverflowDropOldest
	}
}

func (s *CaptureState) Clone() *CaptureState {
	if s == nil {
		return nil
	}
	cp := *s
	cp.NormalizeTimes()
	return &cp
}

func SetCaptureState(c *gin.Context, state *CaptureState) {
	if c == nil || state == nil {
		return
	}
	cp := state.Clone()
	c.Set(captureStateKey, cp)
}

func CaptureStateFromContext(c *gin.Context) (*CaptureState, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(captureStateKey)
	if !ok {
		return nil, false
	}
	state, ok := v.(*CaptureState)
	if !ok || state == nil {
		return nil, false
	}
	return state, true
}

type BodySnapshot struct {
	Kind         string `json:"kind"`
	Body         []byte `json:"body"`
	OriginalSize int    `json:"original_size"`
	Truncated    bool   `json:"truncated"`
	SHA256       string `json:"sha256,omitempty"`
}

func RequestBodySnapshot(c *gin.Context) (*BodySnapshot, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(requestBodySnapshotKey)
	if !ok {
		return nil, false
	}
	snap, ok := v.(*BodySnapshot)
	if !ok || snap == nil {
		return nil, false
	}
	cp := *snap
	cp.Body = cloneBytes(snap.Body)
	return &cp, true
}

func MaybeCaptureRequestBody(c *gin.Context, body []byte, contentType string) {
	if c == nil {
		return
	}
	state, ok := CaptureStateFromContext(c)
	if !ok || state == nil {
		return
	}
	state.NormalizeTimes()
	if !state.ExpiresAt.IsZero() && time.Now().After(state.ExpiresAt) {
		return
	}

	kind := classifyBody(contentType, body)
	if kind == BodyKindBinary {
		snap := binarySnapshot(body, contentType)
		c.Set(requestBodySnapshotKey, snap)
		return
	}

	capBytes := state.SingleRequestCap
	if capBytes <= 0 {
		capBytes = len(body)
	}
	clipped, truncated := TruncateUTF8Bytes(body, capBytes)
	c.Set(requestBodySnapshotKey, &BodySnapshot{
		Kind:         BodyKindText,
		Body:         clipped,
		OriginalSize: len(body),
		Truncated:    truncated,
	})
}

func classifyBody(contentType string, body []byte) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch {
	case ct == "":
		if utf8.Valid(body) {
			return BodyKindText
		}
		return BodyKindBinary
	case ct == "application/json",
		ct == "application/x-ndjson",
		ct == "application/xml",
		ct == "application/x-www-form-urlencoded",
		strings.HasPrefix(ct, "text/"),
		strings.Contains(ct, "+json"),
		strings.Contains(ct, "+xml"):
		return BodyKindText
	case strings.HasPrefix(ct, "multipart/"),
		ct == "application/octet-stream",
		strings.HasPrefix(ct, "image/"),
		strings.HasPrefix(ct, "audio/"),
		strings.HasPrefix(ct, "video/"):
		return BodyKindBinary
	default:
		if utf8.Valid(body) {
			return BodyKindText
		}
		return BodyKindBinary
	}
}

func binarySnapshot(body []byte, contentType string) *BodySnapshot {
	sum := sha256.Sum256(body)
	previewLen := defaultBinaryPreviewLen
	if len(body) < previewLen {
		previewLen = len(body)
	}
	meta := map[string]any{
		"kind":         BodyKindBinary,
		"content_type": strings.TrimSpace(contentType),
		"size":         len(body),
		"sha256":       hex.EncodeToString(sum[:]),
		"preview_hex":  hex.EncodeToString(body[:previewLen]),
	}
	encoded, _ := json.Marshal(meta)
	return &BodySnapshot{
		Kind:         BodyKindBinary,
		Body:         encoded,
		OriginalSize: len(body),
		Truncated:    true,
		SHA256:       hex.EncodeToString(sum[:]),
	}
}

type ReqLogEntry struct {
	UserID           int64             `json:"user_id"`
	SessionID        string            `json:"session_id"`
	Seq              int64             `json:"seq"`
	RequestID        string            `json:"request_id,omitempty"`
	ClientReqID      string            `json:"client_request_id,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	InboundEndpoint  string            `json:"inbound_endpoint,omitempty"`
	Model            string            `json:"model,omitempty"`
	Stream           bool              `json:"stream"`
	Transport        string            `json:"transport"`
	StatusCode       int               `json:"status_code"`
	DurationMs       int64             `json:"duration_ms"`
	AccountID        *int64            `json:"account_id,omitempty"`
	Platform         string            `json:"platform,omitempty"`
	ClientIP         string            `json:"client_ip,omitempty"`
	ReqHeaders       map[string]string `json:"req_headers,omitempty"`
	RespHeaders      map[string]string `json:"resp_headers,omitempty"`
	ReqBody          []byte            `json:"-"`
	ReqBodyKind      string            `json:"req_body_kind"`
	ReqTruncated     bool              `json:"req_truncated"`
	RespBody         []byte            `json:"-"`
	RespTruncated    bool              `json:"resp_truncated"`
	ResponseCaptured bool              `json:"response_captured"`
	ErrorDetail      string            `json:"error_detail,omitempty"`
}

type reqLogEntryJSON struct {
	UserID           int64             `json:"user_id"`
	SessionID        string            `json:"session_id"`
	Seq              int64             `json:"seq"`
	RequestID        string            `json:"request_id,omitempty"`
	ClientReqID      string            `json:"client_request_id,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	InboundEndpoint  string            `json:"inbound_endpoint,omitempty"`
	Model            string            `json:"model,omitempty"`
	Stream           bool              `json:"stream"`
	Transport        string            `json:"transport"`
	StatusCode       int               `json:"status_code"`
	DurationMs       int64             `json:"duration_ms"`
	AccountID        *int64            `json:"account_id,omitempty"`
	Platform         string            `json:"platform,omitempty"`
	ClientIP         string            `json:"client_ip,omitempty"`
	ReqHeaders       map[string]string `json:"req_headers,omitempty"`
	RespHeaders      map[string]string `json:"resp_headers,omitempty"`
	ReqBody          string            `json:"req_body"`
	ReqBodyKind      string            `json:"req_body_kind"`
	ReqTruncated     bool              `json:"req_truncated"`
	RespBody         string            `json:"resp_body"`
	RespTruncated    bool              `json:"resp_truncated"`
	ResponseCaptured bool              `json:"response_captured"`
	ErrorDetail      string            `json:"error_detail,omitempty"`
}

func (e *ReqLogEntry) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}
	return json.Marshal(reqLogEntryJSON{
		UserID:           e.UserID,
		SessionID:        e.SessionID,
		Seq:              e.Seq,
		RequestID:        e.RequestID,
		ClientReqID:      e.ClientReqID,
		Timestamp:        e.Timestamp,
		Method:           e.Method,
		Path:             e.Path,
		InboundEndpoint:  e.InboundEndpoint,
		Model:            e.Model,
		Stream:           e.Stream,
		Transport:        e.Transport,
		StatusCode:       e.StatusCode,
		DurationMs:       e.DurationMs,
		AccountID:        cloneInt64Ptr(e.AccountID),
		Platform:         e.Platform,
		ClientIP:         e.ClientIP,
		ReqHeaders:       cloneStringMap(e.ReqHeaders),
		RespHeaders:      cloneStringMap(e.RespHeaders),
		ReqBody:          string(e.ReqBody),
		ReqBodyKind:      e.ReqBodyKind,
		ReqTruncated:     e.ReqTruncated,
		RespBody:         string(e.RespBody),
		RespTruncated:    e.RespTruncated,
		ResponseCaptured: e.ResponseCaptured,
		ErrorDetail:      e.ErrorDetail,
	})
}

func (e *ReqLogEntry) UnmarshalJSON(raw []byte) error {
	var dto reqLogEntryJSON
	if err := json.Unmarshal(raw, &dto); err != nil {
		return err
	}
	e.UserID = dto.UserID
	e.SessionID = dto.SessionID
	e.Seq = dto.Seq
	e.RequestID = dto.RequestID
	e.ClientReqID = dto.ClientReqID
	e.Timestamp = dto.Timestamp
	e.Method = dto.Method
	e.Path = dto.Path
	e.InboundEndpoint = dto.InboundEndpoint
	e.Model = dto.Model
	e.Stream = dto.Stream
	e.Transport = dto.Transport
	e.StatusCode = dto.StatusCode
	e.DurationMs = dto.DurationMs
	e.AccountID = cloneInt64Ptr(dto.AccountID)
	e.Platform = dto.Platform
	e.ClientIP = dto.ClientIP
	e.ReqHeaders = cloneStringMap(dto.ReqHeaders)
	e.RespHeaders = cloneStringMap(dto.RespHeaders)
	e.ReqBody = []byte(dto.ReqBody)
	e.ReqBodyKind = dto.ReqBodyKind
	e.ReqTruncated = dto.ReqTruncated
	e.RespBody = []byte(dto.RespBody)
	e.RespTruncated = dto.RespTruncated
	e.ResponseCaptured = dto.ResponseCaptured
	e.ErrorDetail = dto.ErrorDetail
	return nil
}

func (e *ReqLogEntry) DeepCopy() *ReqLogEntry {
	if e == nil {
		return nil
	}
	cp := *e
	cp.AccountID = cloneInt64Ptr(e.AccountID)
	cp.ReqHeaders = cloneStringMap(e.ReqHeaders)
	cp.RespHeaders = cloneStringMap(e.RespHeaders)
	cp.ReqBody = cloneBytes(e.ReqBody)
	cp.RespBody = cloneBytes(e.RespBody)
	return &cp
}

func (e *ReqLogEntry) EstimateBytes() int64 {
	if e == nil {
		return 0
	}
	n := len(e.ReqBody) + len(e.RespBody) + len(e.Method) + len(e.Path) + len(e.Model) + len(e.Platform) + len(e.ErrorDetail)
	for k, v := range e.ReqHeaders {
		n += len(k) + len(v)
	}
	for k, v := range e.RespHeaders {
		n += len(k) + len(v)
	}
	return int64(n + 512)
}

func TruncateUTF8Bytes(in []byte, capBytes int) ([]byte, bool) {
	if capBytes < 0 {
		capBytes = 0
	}
	if capBytes == 0 {
		return []byte{}, len(in) > 0
	}
	if len(in) <= capBytes {
		return cloneBytes(in), false
	}
	out := cloneBytes(in[:capBytes])
	for len(out) > 0 && !utf8.Valid(out) {
		out = out[:len(out)-1]
	}
	return out, true
}

func HeaderMap(h http.Header, maxBytes int) map[string]string {
	if len(h) == 0 || maxBytes == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(h))
	used := 0
	for k, values := range h {
		v := strings.Join(values, ", ")
		if maxBytes > 0 && used+len(k)+len(v) > maxBytes {
			out[k] = "<truncated>"
			break
		}
		out[k] = v
		used += len(k) + len(v)
	}
	return out
}

func cloneBytes(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func CompactJSON(raw []byte) []byte {
	if !json.Valid(raw) {
		return raw
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw
	}
	return buf.Bytes()
}
