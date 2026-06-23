package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserURL  = "https://openidconnect.googleapis.com/v1/userinfo"
)

// GoogleProvider implements OAuth login via Google OpenID Connect.
type GoogleProvider struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

func NewGoogleProvider(clientID, clientSecret string) *GoogleProvider {
	return &GoogleProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *GoogleProvider) Name() string {
	return GoogleProviderName
}

func (p *GoogleProvider) AuthCodeURL(state, redirectURI string) string {
	params := url.Values{
		"client_id":     {p.clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"access_type":   {"online"},
		"prompt":        {"select_account"},
	}
	return googleAuthURL + "?" + params.Encode()
}

func (p *GoogleProvider) Exchange(ctx context.Context, code, redirectURI string) (*UserIdentity, error) {
	token, err := p.exchangeCode(ctx, code, redirectURI)
	if err != nil {
		return nil, err
	}
	return p.fetchUserInfo(ctx, token)
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (p *GoogleProvider) exchangeCode(ctx context.Context, code, redirectURI string) (string, error) {
	data := url.Values{
		"client_id":     {p.clientID},
		"client_secret": {p.clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed: %s", string(body))
	}

	var result googleTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token: %s", string(body))
	}
	return result.AccessToken, nil
}

func (p *GoogleProvider) fetchUserInfo(ctx context.Context, accessToken string) (*UserIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo failed: %s", string(body))
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("invalid userinfo response: %w", err)
	}
	if info.Sub == "" || info.Email == "" {
		return nil, fmt.Errorf("incomplete userinfo: %s", string(body))
	}

	return &UserIdentity{
		Subject:       info.Sub,
		Email:         strings.ToLower(strings.TrimSpace(info.Email)),
		EmailVerified: info.EmailVerified,
		Name:          info.Name,
		AvatarURL:     info.Picture,
	}, nil
}
