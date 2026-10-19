//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const responsesMissingModelBody = `{"error":{"code":"model_not_found","message":"unknown provider for model gpt-6-luna","param":"model","type":"invalid_request_error"}}`
const responsesHealthyBody = `{"id":"resp_healthy","object":"response","model":"gpt-6-luna","status":"completed","output":[{"id":"msg_healthy","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"APIPOOL_OK","annotations":[]}]}],"usage":{"input_tokens":10,"output_tokens":8,"total_tokens":18}}`

type responsesModelFailoverUpstream struct {
	service.HTTPUpstream
	mu         sync.Mutex
	hits       []int64
	firstError string
	allFail    bool
}

func (u *responsesModelFailoverUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.mu.Lock()
	u.hits = append(u.hits, accountID)
	u.mu.Unlock()
	status, contentType, payload := http.StatusOK, "application/json", responsesHealthyBody
	if accountID == 225 || u.allFail {
		status, payload = http.StatusBadRequest, u.firstError
	} else if gjson.GetBytes(body, "stream").Bool() {
		contentType = "text/event-stream"
		payload = "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_healthy\",\"status\":\"in_progress\",\"output\":[]}}\n\n" +
			"data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"msg_healthy\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[]}}\n\n" +
			"data: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_healthy\",\"output_index\":0,\"content_index\":0,\"delta\":\"APIPOOL_OK\"}\n\n" +
			"data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":" + gjson.Get(responsesHealthyBody, "output.0").Raw + "}\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":" + responsesHealthyBody + "}\n\n"
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(payload))}, nil
}

func newResponsesModelFailoverRouter(t *testing.T, upstream *responsesModelFailoverUpstream) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupID := int64(901)
	repo := &grokCredentialHandlerRepo{accounts: []service.Account{
		{ID: 225, Name: "missing-model", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, Credentials: map[string]any{"api_key": "test-first", "base_url": "https://first.example.invalid", "pool_mode": true}, Extra: map[string]any{"openai_passthrough": true}},
		{ID: 221, Name: "healthy", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: 2, Credentials: map[string]any{"api_key": "test-second", "base_url": "https://second.example.invalid"}, Extra: map[string]any{"openai_passthrough": true}},
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.MaxAccountSwitches = 3
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	cache := &concurrencyCacheMock{acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil }, acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil }}
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(cache), billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
	key := &service.APIKey{ID: 902, GroupID: &groupID, User: &service.User{ID: 903, Status: service.StatusActive}, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), key)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.User.ID, Concurrency: 1})
		c.Next()
	})
	router.POST("/v1/responses", h.Responses)
	return router
}

func TestResponsesPassthroughModelNotFoundSwitchesAccount(t *testing.T) {
	for _, stream := range []bool{true, false} {
		name, streamJSON := "nonstream", "false"
		if stream {
			name, streamJSON = "stream", "true"
		}
		t.Run(name, func(t *testing.T) {
			upstream := &responsesModelFailoverUpstream{firstError: responsesMissingModelBody}
			router := newResponsesModelFailoverRouter(t, upstream)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-6-luna","input":"hello","stream":`+streamJSON+`}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, []int64{225, 221}, upstream.hits, "missing model must switch accounts without retrying the same pool account")
			require.Contains(t, rec.Body.String(), "APIPOOL_OK")
			require.NotContains(t, rec.Body.String(), "unknown provider")
			require.NotContains(t, rec.Body.String(), "response.failed")
			if stream {
				require.Equal(t, 1, strings.Count(rec.Body.String(), `"type":"response.completed"`))
			}
		})
	}
}

func TestResponsesPassthroughModelFailoverStops(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		allFail    bool
		wantHits   []int64
	}{
		{"invalid parameter", `{"error":{"code":"invalid_function_parameters","message":"Invalid tool schema"}}`, false, []int64{225}},
		{"all models missing", responsesMissingModelBody, true, []int64{225, 221}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &responsesModelFailoverUpstream{firstError: tc.body, allFail: tc.allFail}
			router := newResponsesModelFailoverRouter(t, upstream)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-6-luna","input":"hello","stream":true}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Equal(t, tc.wantHits, upstream.hits)
			require.NotContains(t, rec.Body.String(), "response.completed")
		})
	}
}
