package reqlog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMaybeCaptureRequestBody_TextTruncatesAndCopies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	SetCaptureState(c, &CaptureState{
		UserID:           7,
		SessionID:        "rl_test",
		ExpiresAt:        time.Now().Add(time.Minute),
		SingleRequestCap: 5,
	})

	body := []byte("hello world")
	MaybeCaptureRequestBody(c, body, "application/json")
	body[0] = 'X'

	snap, ok := RequestBodySnapshot(c)
	require.True(t, ok)
	require.Equal(t, BodyKindText, snap.Kind)
	require.Equal(t, []byte("hello"), snap.Body)
	require.True(t, snap.Truncated)
	require.Equal(t, 11, snap.OriginalSize)
}

func TestMaybeCaptureRequestBody_BinaryStoresMetadataOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	SetCaptureState(c, &CaptureState{
		UserID:           9,
		SessionID:        "rl_binary",
		ExpiresAt:        time.Now().Add(time.Minute),
		SingleRequestCap: 1024,
	})

	raw := []byte{0x00, 0x01, 0x02, 0xff}
	MaybeCaptureRequestBody(c, raw, "multipart/form-data; boundary=x")

	snap, ok := RequestBodySnapshot(c)
	require.True(t, ok)
	require.Equal(t, BodyKindBinary, snap.Kind)
	require.NotContains(t, string(snap.Body), string(raw))
	require.Contains(t, string(snap.Body), `"sha256"`)
	require.Contains(t, string(snap.Body), `"size":4`)
}

func TestCaptureWriterTransparentWriteStringAndFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		original := c.Writer
		w := AcquireCaptureWriter(original, 8)
		c.Writer = w
		defer func() {
			if c.Writer == w {
				c.Writer = original
			}
			ReleaseCaptureWriter(w)
		}()
		c.Next()
		require.Equal(t, "hello wo", string(w.CapturedCopy()))
		require.True(t, w.Truncated())
	})
	router.GET("/", func(c *gin.Context) {
		c.Header("X-Test", "ok")
		_, _ = c.Writer.WriteString("hello ")
		_, _ = c.Writer.Write([]byte("world"))
		c.Writer.Flush()
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "hello world", rec.Body.String())
	require.Equal(t, "ok", rec.Header().Get("X-Test"))
	require.True(t, strings.Contains(rec.Body.String(), "world"))
}
