package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountHandler_Create_KiroAcceptsType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	router := gin.New()
	router.POST("/api/v1/admin/accounts", handler.Create)

	body := map[string]any{
		"name":     "kiro-social-1",
		"platform": "anthropic",
		"type":     "kiro",
		"credentials": map[string]any{
			"auth_method":   "social",
			"refresh_token": "rt-1",
			"auth_region":   "us-east-1",
			"api_region":    "us-east-1",
		},
		"concurrency": 1,
		"priority":    1,
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Equal(t, "kiro", adminSvc.createdAccounts[0].Type)
}
