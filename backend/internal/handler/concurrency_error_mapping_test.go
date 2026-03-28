package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func requireErrorObject(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	rawErrObj, ok := body["error"]
	require.True(t, ok, "response body should contain error")

	errObj, ok := rawErrObj.(map[string]any)
	require.True(t, ok, "error field should be an object")

	return errObj
}

func TestMapConcurrencyAcquireError(t *testing.T) {
	t.Run("并发超时仍返回429", func(t *testing.T) {
		status, errType, message := mapConcurrencyAcquireError(&ConcurrencyError{
			SlotType:  "user",
			IsTimeout: true,
		}, "user")

		require.Equal(t, http.StatusTooManyRequests, status)
		require.Equal(t, "rate_limit_error", errType)
		require.Equal(t, "Concurrency limit exceeded for user, please retry later", message)
	})

	t.Run("基础设施错误改为503", func(t *testing.T) {
		status, errType, message := mapConcurrencyAcquireError(errors.New("dial tcp: lookup redis on 127.0.0.11:53: server misbehaving"), "user")

		require.Equal(t, http.StatusServiceUnavailable, status)
		require.Equal(t, "api_error", errType)
		require.Equal(t, "Concurrency system unavailable (cache or network issue), please retry later", message)
	})
}

func TestOpenAIHandleConcurrencyError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("并发超时保持429", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)

		h := &OpenAIGatewayHandler{}
		h.handleConcurrencyError(c, &ConcurrencyError{SlotType: "user", IsTimeout: true}, "user", false)

		require.Equal(t, http.StatusTooManyRequests, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		errObj := requireErrorObject(t, body)
		require.Equal(t, "rate_limit_error", errObj["type"])
		require.Equal(t, "Concurrency limit exceeded for user, please retry later", errObj["message"])
	})

	t.Run("redis异常改为503", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)

		h := &OpenAIGatewayHandler{}
		h.handleConcurrencyError(c, errors.New("dial tcp: lookup redis on 127.0.0.11:53: server misbehaving"), "user", false)

		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		errObj := requireErrorObject(t, body)
		require.Equal(t, "api_error", errObj["type"])
		require.Equal(t, "Concurrency system unavailable (cache or network issue), please retry later", errObj["message"])
	})
}

func TestGatewayHandleConcurrencyError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	h := &GatewayHandler{}
	h.handleConcurrencyError(c, errors.New("dial tcp: lookup redis on 127.0.0.11:53: server misbehaving"), "account", false)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "error", body["type"])
	errObj := requireErrorObject(t, body)
	require.Equal(t, "api_error", errObj["type"])
	require.Equal(t, "Concurrency system unavailable (cache or network issue), please retry later", errObj["message"])
}

func TestSoraHandleConcurrencyError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	h := &SoraGatewayHandler{}
	h.handleConcurrencyError(c, errors.New("dial tcp: lookup redis on 127.0.0.11:53: server misbehaving"), "user", false)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	errObj := requireErrorObject(t, body)
	require.Equal(t, "api_error", errObj["type"])
	require.Equal(t, "Concurrency system unavailable (cache or network issue), please retry later", errObj["message"])
}

func TestMapGeminiConcurrencyAcquireError(t *testing.T) {
	t.Run("并发超时保持429", func(t *testing.T) {
		status, message := mapGeminiConcurrencyAcquireError(&ConcurrencyError{
			SlotType:  "account",
			IsTimeout: true,
		}, "account")

		require.Equal(t, http.StatusTooManyRequests, status)
		require.Equal(t, "Concurrency limit exceeded for account, please retry later", message)
	})

	t.Run("基础设施错误改为503", func(t *testing.T) {
		status, message := mapGeminiConcurrencyAcquireError(errors.New("dial tcp: lookup redis on 127.0.0.11:53: server misbehaving"), "account")

		require.Equal(t, http.StatusServiceUnavailable, status)
		require.Equal(t, "Concurrency system unavailable (cache or network issue), please retry later", message)
	})
}
