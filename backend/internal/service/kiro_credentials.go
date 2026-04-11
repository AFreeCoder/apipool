package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	kiroAuthMethodSocial = "social"
	kiroAuthMethodIDC    = "idc"
	defaultKiroRegion    = "us-east-1"
)

type KiroCredentials struct {
	AccessToken  string
	RefreshToken string
	AuthMethod   string
	ClientID     string
	ClientSecret string
	ProfileARN   string
	Region       string
	AuthRegion   string
	APIRegion    string
	MachineID    string
	BaseURL      string
}

func ParseKiroCredentials(account *Account) (*KiroCredentials, error) {
	if account == nil || !account.IsKiro() {
		return nil, fmt.Errorf("not a kiro account")
	}

	authMethod := NormalizeKiroAuthMethod(account.GetCredential("auth_method"))
	if authMethod == "" {
		return nil, fmt.Errorf("kiro auth_method is required")
	}

	refreshToken := strings.TrimSpace(account.GetCredential("refresh_token"))
	if refreshToken == "" {
		return nil, fmt.Errorf("kiro refresh_token is required")
	}

	machineID, err := NormalizeKiroMachineID(account.GetCredential("machine_id"), refreshToken)
	if err != nil {
		return nil, err
	}

	creds := &KiroCredentials{
		AccessToken:  strings.TrimSpace(account.GetCredential("access_token")),
		RefreshToken: refreshToken,
		AuthMethod:   authMethod,
		ClientID:     strings.TrimSpace(account.GetCredential("client_id")),
		ClientSecret: strings.TrimSpace(account.GetCredential("client_secret")),
		ProfileARN:   strings.TrimSpace(account.GetCredential("profile_arn")),
		Region:       strings.TrimSpace(account.GetCredential("region")),
		AuthRegion:   strings.TrimSpace(account.GetCredential("auth_region")),
		APIRegion:    strings.TrimSpace(account.GetCredential("api_region")),
		MachineID:    machineID,
		BaseURL:      strings.TrimSpace(account.GetCredential("base_url")),
	}

	if creds.AuthMethod == kiroAuthMethodIDC && (creds.ClientID == "" || creds.ClientSecret == "") {
		return nil, fmt.Errorf("kiro idc requires client_id and client_secret")
	}

	return creds, nil
}

func NormalizeKiroAuthMethod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case kiroAuthMethodSocial:
		return kiroAuthMethodSocial
	case kiroAuthMethodIDC, "builder-id", "iam":
		return kiroAuthMethodIDC
	default:
		return ""
	}
}

func NormalizeKiroMachineID(raw string, refreshToken string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		sum := sha256.Sum256([]byte("KotlinNativeAPI/" + refreshToken))
		return hex.EncodeToString(sum[:]), nil
	}

	normalized := strings.ReplaceAll(trimmed, "-", "")
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", fmt.Errorf("invalid kiro machine_id")
	}
	switch len(normalized) {
	case 32:
		return normalized + normalized, nil
	case 64:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid kiro machine_id")
	}
}

func (k *KiroCredentials) EffectiveAuthRegion() string {
	if strings.TrimSpace(k.AuthRegion) != "" {
		return strings.TrimSpace(k.AuthRegion)
	}
	if strings.TrimSpace(k.Region) != "" {
		return strings.TrimSpace(k.Region)
	}
	return defaultKiroRegion
}

func (k *KiroCredentials) EffectiveAPIRegion() string {
	if strings.TrimSpace(k.APIRegion) != "" {
		return strings.TrimSpace(k.APIRegion)
	}
	return defaultKiroRegion
}
