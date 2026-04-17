package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	kiroSocialOAuthRedirectURI    = "http://localhost:49153/callback"
	kiroDesktopUserAgentVersion   = "0.6.18"
	kiroSocialOAuthSessionTimeout = 30 * time.Minute
)

type KiroGenerateSocialAuthURLInput struct {
	ProxyID    *int64
	Provider   string
	AuthRegion string
	APIRegion  string
}

type KiroSocialAuthURLResult struct {
	AuthURL     string `json:"auth_url"`
	SessionID   string `json:"session_id"`
	State       string `json:"state"`
	MachineID   string `json:"machine_id"`
	RedirectURI string `json:"redirect_uri"`
	Provider    string `json:"provider"`
	AuthRegion  string `json:"auth_region"`
	APIRegion   string `json:"api_region"`
}

type KiroExchangeSocialCodeInput struct {
	SessionID string
	State     string
	Code      string
	ProxyID   *int64
}

type KiroSocialTokenInfo struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	ExpiresIn    int64  `json:"expires_in"`
	ProfileARN   string `json:"profile_arn,omitempty"`
	AuthMethod   string `json:"auth_method"`
	Provider     string `json:"provider"`
	AuthRegion   string `json:"auth_region"`
	APIRegion    string `json:"api_region"`
	MachineID    string `json:"machine_id"`
}

type kiroSocialOAuthSession struct {
	State        string
	CodeVerifier string
	ProxyURL     string
	Provider     string
	AuthRegion   string
	APIRegion    string
	MachineID    string
	RedirectURI  string
	CreatedAt    time.Time
}

type kiroSocialOAuthSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*kiroSocialOAuthSession
}

func newKiroSocialOAuthSessionStore() *kiroSocialOAuthSessionStore {
	return &kiroSocialOAuthSessionStore{
		sessions: make(map[string]*kiroSocialOAuthSession),
	}
}

func (s *kiroSocialOAuthSessionStore) Set(sessionID string, session *kiroSocialOAuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

func (s *kiroSocialOAuthSessionStore) Get(sessionID string) (*kiroSocialOAuthSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(session.CreatedAt) > kiroSocialOAuthSessionTimeout {
		return nil, false
	}
	return session, true
}

func (s *kiroSocialOAuthSessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *KiroAuthService) GenerateSocialAuthURL(ctx context.Context, input *KiroGenerateSocialAuthURLInput) (*KiroSocialAuthURLResult, error) {
	if s == nil {
		return nil, fmt.Errorf("kiro auth service is nil")
	}

	provider, err := normalizeKiroSocialProvider(input)
	if err != nil {
		return nil, err
	}

	state, err := generateKiroRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}
	sessionID, err := generateKiroRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	codeVerifier, err := generateKiroCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}
	machineID, err := generateKiroMachineID()
	if err != nil {
		return nil, fmt.Errorf("generate machine id: %w", err)
	}

	authRegion := strings.TrimSpace(input.AuthRegion)
	if authRegion == "" {
		authRegion = defaultKiroRegion
	}
	apiRegion := strings.TrimSpace(input.APIRegion)
	if apiRegion == "" {
		apiRegion = defaultKiroRegion
	}

	proxyURL := ""
	if s.proxyRepo != nil && input != nil && input.ProxyID != nil {
		if proxy, proxyErr := s.proxyRepo.GetByID(ctx, *input.ProxyID); proxyErr == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	session := &kiroSocialOAuthSession{
		State:        state,
		CodeVerifier: codeVerifier,
		ProxyURL:     proxyURL,
		Provider:     provider,
		AuthRegion:   authRegion,
		APIRegion:    apiRegion,
		MachineID:    machineID,
		RedirectURI:  kiroSocialOAuthRedirectURI,
		CreatedAt:    time.Now(),
	}
	s.socialSessionStore.Set(sessionID, session)

	params := url.Values{}
	params.Set("idp", provider)
	params.Set("redirect_uri", session.RedirectURI)
	params.Set("code_challenge", generateKiroCodeChallenge(codeVerifier))
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)

	return &KiroSocialAuthURLResult{
		AuthURL:     fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/login?%s", authRegion, params.Encode()),
		SessionID:   sessionID,
		State:       state,
		MachineID:   machineID,
		RedirectURI: session.RedirectURI,
		Provider:    provider,
		AuthRegion:  authRegion,
		APIRegion:   apiRegion,
	}, nil
}

func (s *KiroAuthService) ExchangeSocialCode(ctx context.Context, input *KiroExchangeSocialCodeInput) (*KiroSocialTokenInfo, error) {
	if s == nil {
		return nil, fmt.Errorf("kiro auth service is nil")
	}
	if input == nil {
		return nil, fmt.Errorf("exchange input is required")
	}

	session, ok := s.socialSessionStore.Get(strings.TrimSpace(input.SessionID))
	if !ok {
		return nil, fmt.Errorf("session not found or expired")
	}
	if strings.TrimSpace(input.State) == "" || strings.TrimSpace(input.State) != session.State {
		return nil, fmt.Errorf("invalid oauth state")
	}
	if strings.TrimSpace(input.Code) == "" {
		return nil, fmt.Errorf("auth code is required")
	}

	proxyURL := session.ProxyURL
	if s.proxyRepo != nil && input.ProxyID != nil {
		if proxy, proxyErr := s.proxyRepo.GetByID(ctx, *input.ProxyID); proxyErr == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	body, err := json.Marshal(map[string]string{
		"code":          strings.TrimSpace(input.Code),
		"code_verifier": session.CodeVerifier,
		"redirect_uri":  session.RedirectURI,
	})
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/oauth/token", session.AuthRegion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", buildKiroDesktopUserAgent(session.MachineID))
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("Connection", kiroRefreshConnectionHeader)

	resp, err := s.httpUpstream.Do(req, proxyURL, 0, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	tokenInfo, err := parseKiroTokenResponse(resp)
	if err != nil {
		return nil, err
	}

	s.socialSessionStore.Delete(strings.TrimSpace(input.SessionID))

	return &KiroSocialTokenInfo{
		AccessToken:  tokenInfo.AccessToken,
		RefreshToken: tokenInfo.RefreshToken,
		ExpiresAt:    tokenInfo.ExpiresAt.Unix(),
		ExpiresIn:    tokenInfo.ExpiresIn,
		ProfileARN:   tokenInfo.ProfileARN,
		AuthMethod:   kiroAuthMethodSocial,
		Provider:     session.Provider,
		AuthRegion:   session.AuthRegion,
		APIRegion:    session.APIRegion,
		MachineID:    session.MachineID,
	}, nil
}

func normalizeKiroSocialProvider(input *KiroGenerateSocialAuthURLInput) (string, error) {
	provider := ""
	if input != nil {
		provider = strings.ToLower(strings.TrimSpace(input.Provider))
	}
	switch provider {
	case "google", "github":
		return provider, nil
	default:
		return "", fmt.Errorf("invalid kiro social provider")
	}
}

func generateKiroMachineID() (string, error) {
	raw, err := generateKiroRandomHex(32)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func generateKiroRandomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func generateKiroCodeVerifier() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateKiroCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func buildKiroDesktopUserAgent(machineID string) string {
	return fmt.Sprintf("KiroIDE-%s-%s", kiroDesktopUserAgentVersion, machineID)
}
