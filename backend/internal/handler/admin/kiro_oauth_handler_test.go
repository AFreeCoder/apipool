package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestKiroOAuthHandler_GenerateAndExchangeCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &kiroOAuthHandlerHTTPUpstreamStub{
		responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"accessToken":"at-2","refreshToken":"rt-2","expiresIn":3600,"profileArn":"arn:aws:kiro:::profile/default"}`),
		},
	}
	svc := service.NewKiroAuthService(nil, upstream)
	handler := NewKiroOAuthHandler(svc)

	router := gin.New()
	router.POST("/api/v1/admin/kiro/oauth/auth-url", handler.GenerateAuthURL)
	router.POST("/api/v1/admin/kiro/oauth/exchange-code", handler.ExchangeCode)

	authReqBody, err := json.Marshal(map[string]any{
		"provider":    "google",
		"auth_region": "us-east-1",
		"api_region":  "us-west-2",
	})
	require.NoError(t, err)

	authRec := httptest.NewRecorder()
	authReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/kiro/oauth/auth-url", bytes.NewReader(authReqBody))
	authReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(authRec, authReq)
	require.Equal(t, http.StatusOK, authRec.Code)

	var authResp struct {
		Data struct {
			SessionID string `json:"session_id"`
			State     string `json:"state"`
			MachineID string `json:"machine_id"`
			AuthURL   string `json:"auth_url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(authRec.Body.Bytes(), &authResp))
	require.NotEmpty(t, authResp.Data.SessionID)
	require.NotEmpty(t, authResp.Data.State)
	require.NotEmpty(t, authResp.Data.MachineID)
	require.Contains(t, authResp.Data.AuthURL, "prod.us-east-1.auth.desktop.kiro.dev/login")

	exchangeReqBody, err := json.Marshal(map[string]any{
		"session_id": authResp.Data.SessionID,
		"state":      authResp.Data.State,
		"code":       "auth-code",
	})
	require.NoError(t, err)

	exchangeRec := httptest.NewRecorder()
	exchangeReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/kiro/oauth/exchange-code", bytes.NewReader(exchangeReqBody))
	exchangeReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(exchangeRec, exchangeReq)
	require.Equal(t, http.StatusOK, exchangeRec.Code)

	var exchangeResp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AuthMethod   string `json:"auth_method"`
			MachineID    string `json:"machine_id"`
			AuthRegion   string `json:"auth_region"`
			APIRegion    string `json:"api_region"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(exchangeRec.Body.Bytes(), &exchangeResp))
	require.Equal(t, "at-2", exchangeResp.Data.AccessToken)
	require.Equal(t, "rt-2", exchangeResp.Data.RefreshToken)
	require.Equal(t, "social", exchangeResp.Data.AuthMethod)
	require.Equal(t, authResp.Data.MachineID, exchangeResp.Data.MachineID)
	require.Equal(t, "us-east-1", exchangeResp.Data.AuthRegion)
	require.Equal(t, "us-west-2", exchangeResp.Data.APIRegion)
}

type kiroOAuthHandlerHTTPUpstreamStub struct {
	responses []*http.Response
}

func (s *kiroOAuthHandlerHTTPUpstreamStub) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func (s *kiroOAuthHandlerHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

func newJSONResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
}
