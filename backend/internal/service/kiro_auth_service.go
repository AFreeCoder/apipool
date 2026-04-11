package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	kiroSocialAcceptHeader      = "application/json, text/plain, */*"
	kiroSocialAcceptEncoding    = "gzip, compress, deflate, br"
	kiroIDCSDKUserAgent         = "aws-sdk-js/3.980.0 KiroIDE"
	kiroIDCUserAgent            = "aws-sdk-js/3.980.0 ua/2.1 api/sso-oidc#3.980.0 KiroIDE"
	kiroIDCSDKRequestHeader     = "attempt=1; max=4"
	kiroRefreshConnectionHeader = "close"
)

type KiroTokenInfo struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	ExpiresIn    int64
	ProfileARN   string
}

type KiroAuthService struct {
	proxyRepo    ProxyRepository
	httpUpstream HTTPUpstream
}

func NewKiroAuthService(proxyRepo ProxyRepository, httpUpstream HTTPUpstream) *KiroAuthService {
	return &KiroAuthService{
		proxyRepo:    proxyRepo,
		httpUpstream: httpUpstream,
	}
}

func (s *KiroAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*KiroTokenInfo, error) {
	creds, err := ParseKiroCredentials(account)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if s.proxyRepo != nil && account != nil && account.ProxyID != nil {
		if proxy, proxyErr := s.proxyRepo.GetByID(ctx, *account.ProxyID); proxyErr == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	if creds.AuthMethod == kiroAuthMethodSocial {
		return s.refreshSocial(ctx, account, creds, proxyURL)
	}
	return s.refreshIDC(ctx, account, creds, proxyURL)
}

func (s *KiroAuthService) refreshSocial(ctx context.Context, account *Account, creds *KiroCredentials, proxyURL string) (*KiroTokenInfo, error) {
	body, err := json.Marshal(map[string]string{
		"refreshToken": creds.RefreshToken,
	})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/refreshToken", creds.EffectiveAuthRegion())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", kiroSocialAcceptHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", kiroSocialAcceptEncoding)
	req.Header.Set("User-Agent", fmt.Sprintf("KiroIDE-dev-%s", creds.MachineID))
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("Connection", kiroRefreshConnectionHeader)

	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	info, err := parseKiroRefreshResponse(resp)
	if err != nil {
		return nil, err
	}
	if info.RefreshToken == "" {
		info.RefreshToken = creds.RefreshToken
	}
	if info.ProfileARN == "" {
		info.ProfileARN = creds.ProfileARN
	}
	return info, nil
}

func (s *KiroAuthService) refreshIDC(ctx context.Context, account *Account, creds *KiroCredentials, proxyURL string) (*KiroTokenInfo, error) {
	body, err := json.Marshal(map[string]string{
		"clientId":     creds.ClientID,
		"clientSecret": creds.ClientSecret,
		"refreshToken": creds.RefreshToken,
		"grantType":    "refresh_token",
	})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", creds.EffectiveAuthRegion())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-amz-user-agent", kiroIDCSDKUserAgent)
	req.Header.Set("user-agent", kiroIDCUserAgent)
	req.Header.Set("host", req.URL.Host)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", kiroIDCSDKRequestHeader)
	req.Header.Set("Connection", kiroRefreshConnectionHeader)

	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	info, err := parseKiroRefreshResponse(resp)
	if err != nil {
		return nil, err
	}
	if info.RefreshToken == "" {
		info.RefreshToken = creds.RefreshToken
	}
	if info.ProfileARN == "" {
		info.ProfileARN = creds.ProfileARN
	}
	return info, nil
}

func parseKiroRefreshResponse(resp *http.Response) (*KiroTokenInfo, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bodyText := strings.TrimSpace(string(body))
	if resp.StatusCode >= http.StatusBadRequest {
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(bodyText), "invalid_grant") {
			return nil, fmt.Errorf("invalid_grant: %s", bodyText)
		}
		return nil, fmt.Errorf("kiro refresh failed: %s", bodyText)
	}

	var raw struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int64  `json:"expiresIn"`
		ProfileARN   string `json:"profileArn"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw.AccessToken) == "" {
		return nil, fmt.Errorf("kiro refresh response missing accessToken")
	}

	info := &KiroTokenInfo{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresIn:    raw.ExpiresIn,
		ProfileARN:   raw.ProfileARN,
	}
	if raw.ExpiresIn > 0 {
		info.ExpiresAt = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	}
	return info, nil
}
