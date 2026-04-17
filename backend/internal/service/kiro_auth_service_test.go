//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type kiroAuthHTTPUpstreamStub struct {
	requests  []*http.Request
	bodies    []string
	responses []*http.Response
}

func (s *kiroAuthHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(strings.NewReader(string(body)))
	s.requests = append(s.requests, req)
	s.bodies = append(s.bodies, string(body))
	if len(s.responses) == 0 {
		return nil, io.EOF
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func (s *kiroAuthHTTPUpstreamStub) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, "", 0, 0)
}

func TestKiroAuthService_RefreshSocial(t *testing.T) {
	t.Parallel()

	upstream := &kiroAuthHTTPUpstreamStub{
		responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"accessToken":"at-2","refreshToken":"rt-2","expiresIn":3600,"profileArn":"arn:aws:kiro:::profile/default"}`),
		},
	}
	svc := NewKiroAuthService(nil, upstream)
	account := &Account{
		ID:          1,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"auth_method":   "social",
			"refresh_token": "rt-1",
			"auth_region":   "us-east-1",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "at-2", info.AccessToken)
	require.Equal(t, "rt-2", info.RefreshToken)
	require.Equal(t, "arn:aws:kiro:::profile/default", info.ProfileARN)
	require.Equal(t, "application/json", upstream.requests[0].Header.Get("Content-Type"))
	require.Contains(t, upstream.requests[0].Header.Get("User-Agent"), "KiroIDE-")
	require.Contains(t, upstream.bodies[0], `"refreshToken":"rt-1"`)
}

func TestKiroAuthService_GenerateSocialAuthURL(t *testing.T) {
	t.Parallel()

	svc := NewKiroAuthService(nil, &kiroAuthHTTPUpstreamStub{})

	result, err := svc.GenerateSocialAuthURL(context.Background(), &KiroGenerateSocialAuthURLInput{
		Provider:   "google",
		AuthRegion: "us-east-1",
		APIRegion:  "us-west-2",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.AuthURL)
	require.NotEmpty(t, result.SessionID)
	require.NotEmpty(t, result.State)
	require.NotEmpty(t, result.MachineID)
	require.Equal(t, "us-east-1", result.AuthRegion)
	require.Equal(t, "us-west-2", result.APIRegion)

	parsed, err := url.Parse(result.AuthURL)
	require.NoError(t, err)
	require.Equal(t, "https", parsed.Scheme)
	require.Equal(t, "prod.us-east-1.auth.desktop.kiro.dev", parsed.Host)
	require.Equal(t, "/login", parsed.Path)
	require.Equal(t, "google", parsed.Query().Get("idp"))
	require.NotEmpty(t, parsed.Query().Get("redirect_uri"))
	require.Equal(t, result.State, parsed.Query().Get("state"))
	require.NotEmpty(t, parsed.Query().Get("code_challenge"))
}

func TestKiroAuthService_ExchangeSocialCode_StateMismatch(t *testing.T) {
	t.Parallel()

	upstream := &kiroAuthHTTPUpstreamStub{}
	svc := NewKiroAuthService(nil, upstream)

	result, err := svc.GenerateSocialAuthURL(context.Background(), &KiroGenerateSocialAuthURLInput{
		Provider: "github",
	})
	require.NoError(t, err)

	_, err = svc.ExchangeSocialCode(context.Background(), &KiroExchangeSocialCodeInput{
		SessionID: result.SessionID,
		State:     "wrong-state",
		Code:      "auth-code",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "state")
	require.Empty(t, upstream.requests)
}

func TestKiroAuthService_ExchangeSocialCode_Success(t *testing.T) {
	t.Parallel()

	upstream := &kiroAuthHTTPUpstreamStub{
		responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"accessToken":"at-2","refreshToken":"rt-2","expiresIn":3600,"profileArn":"arn:aws:kiro:::profile/default"}`),
		},
	}
	svc := NewKiroAuthService(nil, upstream)

	result, err := svc.GenerateSocialAuthURL(context.Background(), &KiroGenerateSocialAuthURLInput{
		Provider:   "google",
		AuthRegion: "us-east-1",
		APIRegion:  "eu-west-1",
	})
	require.NoError(t, err)

	info, err := svc.ExchangeSocialCode(context.Background(), &KiroExchangeSocialCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "auth-code",
	})
	require.NoError(t, err)
	require.Equal(t, "at-2", info.AccessToken)
	require.Equal(t, "rt-2", info.RefreshToken)
	require.Equal(t, "social", info.AuthMethod)
	require.Equal(t, "google", info.Provider)
	require.Equal(t, "us-east-1", info.AuthRegion)
	require.Equal(t, "eu-west-1", info.APIRegion)
	require.Equal(t, result.MachineID, info.MachineID)
	require.Equal(t, "arn:aws:kiro:::profile/default", info.ProfileARN)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "application/json", upstream.requests[0].Header.Get("Content-Type"))
	require.Contains(t, upstream.requests[0].Header.Get("User-Agent"), "KiroIDE-")
	require.Contains(t, upstream.bodies[0], `"code":"auth-code"`)
	require.Contains(t, upstream.bodies[0], `"redirect_uri":`)
	require.Contains(t, upstream.bodies[0], `"code_verifier":`)
}

func TestKiroAuthService_RefreshIDC_InvalidGrant(t *testing.T) {
	t.Parallel()

	upstream := &kiroAuthHTTPUpstreamStub{
		responses: []*http.Response{
			newJSONResponse(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Invalid refresh token provided"}`),
		},
	}
	svc := NewKiroAuthService(nil, upstream)
	account := &Account{
		ID:          2,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"auth_method":   "idc",
			"refresh_token": "rt-bad",
			"client_id":     "client-1",
			"client_secret": "secret-1",
			"auth_region":   "us-east-1",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_grant")
	require.Equal(t, "application/json", upstream.requests[0].Header.Get("content-type"))
	require.Contains(t, upstream.bodies[0], `"grantType":"refresh_token"`)
}

func TestParseKiroRefreshResponse_ClassifiesErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		statusCode       int
		body             string
		wantKind         KiroRefreshErrorKind
		wantNonRetryable bool
	}{
		{
			name:             "invalid_grant",
			statusCode:       http.StatusBadRequest,
			body:             `{"error":"invalid_grant","error_description":"Invalid refresh token provided"}`,
			wantKind:         kiroRefreshErrorInvalidGrant,
			wantNonRetryable: true,
		},
		{
			name:             "forbidden",
			statusCode:       http.StatusForbidden,
			body:             `{"message":"forbidden"}`,
			wantKind:         kiroRefreshErrorForbidden,
			wantNonRetryable: false,
		},
		{
			name:             "rate_limited",
			statusCode:       http.StatusTooManyRequests,
			body:             `{"message":"slow down"}`,
			wantKind:         kiroRefreshErrorRateLimited,
			wantNonRetryable: false,
		},
		{
			name:             "upstream",
			statusCode:       http.StatusBadGateway,
			body:             `{"message":"bad gateway"}`,
			wantKind:         kiroRefreshErrorUpstream,
			wantNonRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := newJSONResponse(tt.statusCode, tt.body)
			info, err := parseKiroRefreshResponse(resp)
			require.Nil(t, info)
			require.Error(t, err)

			var refreshErr *KiroRefreshError
			require.ErrorAs(t, err, &refreshErr)
			require.Equal(t, tt.statusCode, refreshErr.StatusCode)
			require.Equal(t, tt.wantKind, refreshErr.Kind)
			require.Equal(t, tt.wantNonRetryable, isNonRetryableRefreshError(err))
		})
	}
}

func TestKiroAuthService_RefreshIDC_415FallsBackToFormURLEncoded(t *testing.T) {
	t.Parallel()

	upstream := &kiroAuthHTTPUpstreamStub{
		responses: []*http.Response{
			newJSONResponse(http.StatusUnsupportedMediaType, `{"message":"unsupported media type"}`),
			newJSONResponse(http.StatusOK, `{"accessToken":"at-2","refreshToken":"rt-2","expiresIn":3600,"profileArn":"arn:aws:kiro:::profile/default"}`),
		},
	}
	svc := NewKiroAuthService(nil, upstream)
	account := &Account{
		ID:          3,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"auth_method":   "idc",
			"refresh_token": "rt-1",
			"client_id":     "client-1",
			"client_secret": "secret-1",
			"auth_region":   "us-east-1",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "at-2", info.AccessToken)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "application/json", upstream.requests[0].Header.Get("content-type"))
	require.Contains(t, upstream.requests[1].Header.Get("content-type"), "application/x-www-form-urlencoded")
	require.Contains(t, upstream.bodies[1], "grant_type=refresh_token")
	require.Contains(t, upstream.bodies[1], "client_id=client-1")
	require.Contains(t, upstream.bodies[1], "client_secret=secret-1")
	require.Contains(t, upstream.bodies[1], "refresh_token=rt-1")
}

func TestParseKiroRefreshResponse_429ErrorIncludesStatus(t *testing.T) {
	t.Parallel()

	_, err := parseKiroRefreshResponse(newJSONResponse(http.StatusTooManyRequests, `{"message":"slow down"}`))
	require.Error(t, err)
	require.True(t, errors.Is(err, &KiroRefreshError{Kind: kiroRefreshErrorRateLimited}))
	require.Contains(t, err.Error(), "429")
}
