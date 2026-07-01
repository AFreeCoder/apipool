package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRecordGrokMediaUsage(t *testing.T) {
	tests := []struct {
		name     string
		endpoint service.GrokMediaEndpoint
		model    string
		want     bool
	}{
		{
			name:     "image generation records usage",
			endpoint: service.GrokMediaEndpointImagesGenerations,
			model:    "grok-imagine",
			want:     true,
		},
		{
			name:     "image edit records usage",
			endpoint: service.GrokMediaEndpointImagesEdits,
			model:    "grok-imagine-edit",
			want:     true,
		},
		{
			name:     "video generation records usage",
			endpoint: service.GrokMediaEndpointVideosGenerations,
			model:    "grok-imagine-video-1.5",
			want:     true,
		},
		{
			name:     "video status skips empty model usage",
			endpoint: service.GrokMediaEndpointVideoStatus,
			model:    "",
			want:     false,
		},
		{
			name:     "generation skips usage without model",
			endpoint: service.GrokMediaEndpointImagesGenerations,
			model:    " ",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldRecordGrokMediaUsage(tt.endpoint, tt.model))
		})
	}
}

func TestReadGrokMediaRequestBodyCapturesJSONReqLogSnapshot(t *testing.T) {
	c := newGrokMediaReqLogContext(t, []byte(`{"model":"grok-imagine","prompt":"draw a cube"}`), "application/json")

	body, err := readGrokMediaRequestBody(c, service.GrokMediaEndpointImagesGenerations)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"grok-imagine","prompt":"draw a cube"}`, string(body))

	snap, ok := reqlog.RequestBodySnapshot(c)
	require.True(t, ok)
	require.Equal(t, reqlog.BodyKindText, snap.Kind)
	require.JSONEq(t, `{"model":"grok-imagine","prompt":"draw a cube"}`, string(snap.Body))
	require.False(t, snap.Truncated)
}

func TestReadGrokMediaRequestBodyCapturesMultipartReqLogSnapshot(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "grok-imagine-edit"))
	require.NoError(t, writer.WriteField("prompt", "edit the image"))
	part, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 'P', 'N', 'G'})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c := newGrokMediaReqLogContext(t, buf.Bytes(), writer.FormDataContentType())

	body, err := readGrokMediaRequestBody(c, service.GrokMediaEndpointImagesEdits)
	require.NoError(t, err)
	require.Equal(t, buf.Bytes(), body)

	snap, ok := reqlog.RequestBodySnapshot(c)
	require.True(t, ok)
	require.Equal(t, reqlog.BodyKindBinary, snap.Kind)
	require.Contains(t, string(snap.Body), `"sha256"`)
	require.Contains(t, string(snap.Body), `"size":`)
}

func newGrokMediaReqLogContext(t *testing.T, body []byte, contentType string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	reqlog.SetCaptureState(c, &reqlog.CaptureState{
		UserID:           7,
		SessionID:        "rl_grok_media",
		ExpiresAt:        time.Now().Add(time.Minute),
		SingleRequestCap: 4096,
	})
	return c
}
