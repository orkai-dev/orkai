package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrTokenExpired = errors.New("token has expired")

	oauthStateIssuer        = "orkai-oauth-state"
	oauth2FAChallengeIssuer = "orkai-oauth-2fa"
)

const (
	oauthStateExpiry        = 10 * time.Minute
	oauth2FAChallengeExpiry = 5 * time.Minute
)

// Claims represents the JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID       uuid.UUID `json:"uid"`
	OrgID        uuid.UUID `json:"oid"`
	Role         string    `json:"role"`
	TokenVersion int       `json:"tv"`
}

// RefreshClaims represents claims in a refresh token.
type RefreshClaims struct {
	jwt.RegisteredClaims
	TokenVersion int `json:"tv"`
}

// OAuthStateClaims represents a signed OAuth CSRF state token.
type OAuthStateClaims struct {
	jwt.RegisteredClaims
	Provider string `json:"provider"`
}

// OAuth2FAChallengeClaims represents a short-lived challenge for OAuth 2FA completion.
type OAuth2FAChallengeClaims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"uid"`
}

// JWTManager handles JWT token generation and validation.
type JWTManager struct {
	secret        []byte
	tokenExpiry   time.Duration
	refreshExpiry time.Duration
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secret string, tokenExpiry, refreshExpiry time.Duration) *JWTManager {
	return &JWTManager{
		secret:        []byte(secret),
		tokenExpiry:   tokenExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// GenerateTokenPair creates a new access + refresh token pair.
func (m *JWTManager) GenerateTokenPair(userID, orgID uuid.UUID, role string, tokenVersion ...int) (*TokenPair, error) {
	now := time.Now()

	// Access token
	tv := 0
	if len(tokenVersion) > 0 {
		tv = tokenVersion[0]
	}
	accessClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.tokenExpiry)),
			Issuer:    "orkai",
		},
		UserID:       userID,
		OrgID:        orgID,
		Role:         role,
		TokenVersion: tv,
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(m.secret)
	if err != nil {
		return nil, err
	}

	// Refresh token (longer lived, includes token version for invalidation)
	refreshClaims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiry)),
			Issuer:    "orkai-refresh",
		},
		TokenVersion: tv,
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(m.secret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(m.tokenExpiry),
	}, nil
}

// ValidateAccessToken parses and validates an access token.
func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateRefreshToken parses and validates a refresh token, returning the user ID and token version.
func (m *JWTManager) ValidateRefreshToken(tokenString string) (uuid.UUID, int, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		return uuid.Nil, 0, ErrInvalidToken
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid || claims.Issuer != "orkai-refresh" {
		return uuid.Nil, 0, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, 0, ErrInvalidToken
	}

	return userID, claims.TokenVersion, nil
}

// GenerateOAuthState creates a signed, self-expiring OAuth CSRF state token.
func (m *JWTManager) GenerateOAuthState(provider string) (string, error) {
	now := time.Now()
	claims := OAuthStateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(oauthStateExpiry)),
			Issuer:    oauthStateIssuer,
		},
		Provider: provider,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// ParseOAuthState validates an OAuth state token and ensures it matches the expected provider.
func (m *JWTManager) ParseOAuthState(tokenString, expectedProvider string) error {
	token, err := jwt.ParseWithClaims(tokenString, &OAuthStateClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return ErrTokenExpired
		}
		return ErrInvalidToken
	}

	claims, ok := token.Claims.(*OAuthStateClaims)
	if !ok || !token.Valid || claims.Issuer != oauthStateIssuer {
		return ErrInvalidToken
	}
	if claims.Provider != expectedProvider {
		return ErrInvalidToken
	}
	return nil
}

// GenerateOAuth2FAChallenge creates a short-lived token authorizing OAuth 2FA completion.
func (m *JWTManager) GenerateOAuth2FAChallenge(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := OAuth2FAChallengeClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(oauth2FAChallengeExpiry)),
			Issuer:    oauth2FAChallengeIssuer,
		},
		UserID: userID,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// ValidateOAuth2FAChallenge validates an OAuth 2FA challenge and returns the user
// ID, the token's unique ID (jti) and its expiry. Callers use the jti and expiry
// to enforce single use via durable storage.
func (m *JWTManager) ValidateOAuth2FAChallenge(tokenString string) (uuid.UUID, string, time.Time, error) {
	token, err := jwt.ParseWithClaims(tokenString, &OAuth2FAChallengeClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.Nil, "", time.Time{}, ErrTokenExpired
		}
		return uuid.Nil, "", time.Time{}, ErrInvalidToken
	}

	claims, ok := token.Claims.(*OAuth2FAChallengeClaims)
	if !ok || !token.Valid || claims.Issuer != oauth2FAChallengeIssuer {
		return uuid.Nil, "", time.Time{}, ErrInvalidToken
	}
	userID := claims.UserID
	if userID == uuid.Nil {
		parsed, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, "", time.Time{}, ErrInvalidToken
		}
		userID = parsed
	}

	var expiresAt time.Time
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	return userID, claims.ID, expiresAt, nil
}
