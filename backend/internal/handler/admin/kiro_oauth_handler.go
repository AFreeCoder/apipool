package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type KiroOAuthHandler struct {
	kiroAuthService *service.KiroAuthService
}

func NewKiroOAuthHandler(kiroAuthService *service.KiroAuthService) *KiroOAuthHandler {
	return &KiroOAuthHandler{kiroAuthService: kiroAuthService}
}

type KiroGenerateAuthURLRequest struct {
	ProxyID    *int64 `json:"proxy_id"`
	Provider   string `json:"provider" binding:"required"`
	AuthRegion string `json:"auth_region"`
	APIRegion  string `json:"api_region"`
}

func (h *KiroOAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req KiroGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}

	result, err := h.kiroAuthService.GenerateSocialAuthURL(c.Request.Context(), &service.KiroGenerateSocialAuthURLInput{
		ProxyID:    req.ProxyID,
		Provider:   req.Provider,
		AuthRegion: req.AuthRegion,
		APIRegion:  req.APIRegion,
	})
	if err != nil {
		response.BadRequest(c, "生成 Kiro 授权链接失败: "+err.Error())
		return
	}

	response.Success(c, result)
}

type KiroExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	State     string `json:"state" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

func (h *KiroOAuthHandler) ExchangeCode(c *gin.Context) {
	var req KiroExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}

	tokenInfo, err := h.kiroAuthService.ExchangeSocialCode(c.Request.Context(), &service.KiroExchangeSocialCodeInput{
		SessionID: req.SessionID,
		State:     req.State,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.BadRequest(c, "Kiro token 交换失败: "+err.Error())
		return
	}

	response.Success(c, tokenInfo)
}
