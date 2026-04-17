package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type KiroRefreshErrorKind string

const (
	kiroRefreshErrorInvalidGrant         KiroRefreshErrorKind = "invalid_grant"
	kiroRefreshErrorUnauthorized         KiroRefreshErrorKind = "unauthorized"
	kiroRefreshErrorForbidden            KiroRefreshErrorKind = "forbidden"
	kiroRefreshErrorRateLimited          KiroRefreshErrorKind = "rate_limited"
	kiroRefreshErrorUpstream             KiroRefreshErrorKind = "upstream"
	kiroRefreshErrorUnsupportedMediaType KiroRefreshErrorKind = "unsupported_media_type"
	kiroRefreshErrorFailed               KiroRefreshErrorKind = "failed"
)

type KiroRefreshError struct {
	StatusCode int
	Kind       KiroRefreshErrorKind
	Body       string
}

func (e *KiroRefreshError) Error() string {
	if e == nil {
		return ""
	}

	body := strings.TrimSpace(e.Body)
	switch e.Kind {
	case kiroRefreshErrorInvalidGrant:
		return fmt.Sprintf("invalid_grant: %d %s", e.StatusCode, body)
	case kiroRefreshErrorUnauthorized:
		return fmt.Sprintf("kiro refresh unauthorized: %d %s", e.StatusCode, body)
	case kiroRefreshErrorForbidden:
		return fmt.Sprintf("kiro refresh forbidden: %d %s", e.StatusCode, body)
	case kiroRefreshErrorRateLimited:
		return fmt.Sprintf("kiro refresh rate limited: %d %s", e.StatusCode, body)
	case kiroRefreshErrorUpstream:
		return fmt.Sprintf("kiro refresh upstream error: %d %s", e.StatusCode, body)
	case kiroRefreshErrorUnsupportedMediaType:
		return fmt.Sprintf("kiro refresh unsupported media type: %d %s", e.StatusCode, body)
	default:
		return fmt.Sprintf("kiro refresh failed: %d %s", e.StatusCode, body)
	}
}

func (e *KiroRefreshError) Is(target error) bool {
	other, ok := target.(*KiroRefreshError)
	if !ok {
		return false
	}
	if other.Kind != "" && other.Kind != e.Kind {
		return false
	}
	return other.StatusCode == 0 || other.StatusCode == e.StatusCode
}

type KiroAuthService struct {
	proxyRepo          ProxyRepository
	httpUpstream       HTTPUpstream
	socialSessionStore *kiroSocialOAuthSessionStore
}

func NewKiroAuthService(proxyRepo ProxyRepository, httpUpstream HTTPUpstream) *KiroAuthService {
	return &KiroAuthService{
		proxyRepo:          proxyRepo,
		httpUpstream:       httpUpstream,
		socialSessionStore: newKiroSocialOAuthSessionStore(),
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
	req.Header.Set("User-Agent", buildKiroDesktopUserAgent(creds.MachineID))
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
	info, err := s.refreshIDCWithEncoding(ctx, account, creds, proxyURL, true)
	if err == nil {
		return info, nil
	}

	var refreshErr *KiroRefreshError
	if errors.As(err, &refreshErr) && refreshErr.StatusCode == http.StatusUnsupportedMediaType {
		return s.refreshIDCWithEncoding(ctx, account, creds, proxyURL, false)
	}
	return nil, err
}

func (s *KiroAuthService) refreshIDCWithEncoding(
	ctx context.Context,
	account *Account,
	creds *KiroCredentials,
	proxyURL string,
	useJSON bool,
) (*KiroTokenInfo, error) {
	req, err := buildKiroIDCRefreshRequest(ctx, creds, useJSON)
	if err != nil {
		return nil, err
	}

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

func buildKiroIDCRefreshRequest(ctx context.Context, creds *KiroCredentials, useJSON bool) (*http.Request, error) {
	urlStr := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", creds.EffectiveAuthRegion())
	var (
		body        io.Reader
		contentType string
	)

	if useJSON {
		payload, err := json.Marshal(map[string]string{
			"clientId":     creds.ClientID,
			"clientSecret": creds.ClientSecret,
			"refreshToken": creds.RefreshToken,
			"grantType":    "refresh_token",
		})
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(payload)
		contentType = "application/json"
	} else {
		form := url.Values{}
		form.Set("client_id", creds.ClientID)
		form.Set("client_secret", creds.ClientSecret)
		form.Set("refresh_token", creds.RefreshToken)
		form.Set("grant_type", "refresh_token")
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded; charset=utf-8"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", contentType)
	req.Header.Set("x-amz-user-agent", kiroIDCSDKUserAgent)
	req.Header.Set("user-agent", kiroIDCUserAgent)
	req.Header.Set("host", req.URL.Host)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", kiroIDCSDKRequestHeader)
	req.Header.Set("Connection", kiroRefreshConnectionHeader)
	return req, nil
}

func parseKiroRefreshResponse(resp *http.Response) (*KiroTokenInfo, error) {
	return parseKiroTokenResponse(resp)
}

func parseKiroTokenResponse(resp *http.Response) (*KiroTokenInfo, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bodyText := strings.TrimSpace(string(body))
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, classifyKiroRefreshError(resp.StatusCode, bodyText)
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

func classifyKiroRefreshError(statusCode int, bodyText string) error {
	lowerBody := strings.ToLower(bodyText)
	switch {
	case statusCode == http.StatusBadRequest && strings.Contains(lowerBody, "invalid_grant"):
		return &KiroRefreshError{StatusCode: statusCode, Kind: kiroRefreshErrorInvalidGrant, Body: bodyText}
	case statusCode == http.StatusUnauthorized:
		return &KiroRefreshError{StatusCode: statusCode, Kind: kiroRefreshErrorUnauthorized, Body: bodyText}
	case statusCode == http.StatusForbidden:
		return &KiroRefreshError{StatusCode: statusCode, Kind: kiroRefreshErrorForbidden, Body: bodyText}
	case statusCode == http.StatusTooManyRequests:
		return &KiroRefreshError{StatusCode: statusCode, Kind: kiroRefreshErrorRateLimited, Body: bodyText}
	case statusCode == http.StatusUnsupportedMediaType:
		return &KiroRefreshError{StatusCode: statusCode, Kind: kiroRefreshErrorUnsupportedMediaType, Body: bodyText}
	case statusCode >= http.StatusInternalServerError:
		return &KiroRefreshError{StatusCode: statusCode, Kind: kiroRefreshErrorUpstream, Body: bodyText}
	default:
		return &KiroRefreshError{StatusCode: statusCode, Kind: kiroRefreshErrorFailed, Body: bodyText}
	}
}
