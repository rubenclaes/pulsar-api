package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OAuthConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	TokenURL     string
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"` // seconds
}

type TokenProvider struct {
	cfg    OAuthConfig
	mu     sync.Mutex
	token  string
	expiry time.Time
	client *http.Client
}

func NewTokenProvider(cfg OAuthConfig) *TokenProvider {
	return &TokenProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *TokenProvider) GetToken(ctx context.Context) (string, error) {
	if !p.cfg.Enabled {
		return "", errors.New("oauth not enabled")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Nog geldig? (kleine marge)
	if p.token != "" && time.Now().Before(p.expiry.Add(-30*time.Second)) {
		return p.token, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", errors.New("token endpoint returned non-2xx")
	}

	var tr tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", errors.New("empty access_token")
	}

	p.token = tr.AccessToken
	if tr.ExpiresIn > 0 {
		p.expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		p.expiry = time.Now().Add(5 * time.Minute)
	}

	return p.token, nil
}
