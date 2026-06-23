package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newManager() *JWTManager {
	return NewJWTManager("test-secret", time.Hour, 24*time.Hour)
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	m := newManager()
	userID := uuid.New()
	orgID := uuid.New()

	pair, err := m.GenerateTokenPair(userID, orgID, "admin")
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(time.Hour), pair.ExpiresAt, 5*time.Second)

	claims, err := m.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, orgID, claims.OrgID)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, 0, claims.TokenVersion)
}

func TestGenerateTokenPairAccessTokenVersion(t *testing.T) {
	m := newManager()
	userID := uuid.New()
	pair, err := m.GenerateTokenPair(userID, uuid.New(), "member", 7)
	require.NoError(t, err)

	claims, err := m.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, 7, claims.TokenVersion)
}

func TestGenerateTokenPairWithVersion(t *testing.T) {
	m := newManager()
	userID := uuid.New()
	pair, err := m.GenerateTokenPair(userID, uuid.New(), "member", 7)
	require.NoError(t, err)

	gotUser, tv, err := m.ValidateRefreshToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUser)
	assert.Equal(t, 7, tv)
}

func TestValidateAccessTokenExpired(t *testing.T) {
	m := NewJWTManager("test-secret", -time.Hour, time.Hour) // already expired
	pair, err := m.GenerateTokenPair(uuid.New(), uuid.New(), "admin")
	require.NoError(t, err)

	_, err = m.ValidateAccessToken(pair.AccessToken)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestValidateAccessTokenWrongSecret(t *testing.T) {
	m := newManager()
	pair, err := m.GenerateTokenPair(uuid.New(), uuid.New(), "admin")
	require.NoError(t, err)

	other := NewJWTManager("different-secret", time.Hour, time.Hour)
	_, err = other.ValidateAccessToken(pair.AccessToken)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateAccessTokenMalformed(t *testing.T) {
	m := newManager()
	_, err := m.ValidateAccessToken("not-a-token")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateAccessTokenWrongSigningMethod(t *testing.T) {
	m := newManager()
	// Token signed with "none" algorithm should be rejected by the keyfunc.
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		UserID: uuid.New(),
		Role:   "admin",
	})
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = m.ValidateAccessToken(signed)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateRefreshTokenWrongSigningMethod(t *testing.T) {
	m := newManager()
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "orkai-refresh",
		},
		TokenVersion: 1,
	})
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, _, err = m.ValidateRefreshToken(signed)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateRefreshTokenInvalid(t *testing.T) {
	m := newManager()
	_, _, err := m.ValidateRefreshToken("garbage")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateRefreshTokenWrongIssuer(t *testing.T) {
	m := newManager()
	// An access token has issuer "orkai", not "orkai-refresh".
	pair, err := m.GenerateTokenPair(uuid.New(), uuid.New(), "admin")
	require.NoError(t, err)

	_, _, err = m.ValidateRefreshToken(pair.AccessToken)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateRefreshTokenBadSubject(t *testing.T) {
	m := newManager()
	// Craft a refresh token with a non-UUID subject.
	claims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "not-a-uuid",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "orkai-refresh",
		},
		TokenVersion: 1,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)

	_, _, err = m.ValidateRefreshToken(signed)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestGenerateAndParseOAuthState(t *testing.T) {
	m := newManager()
	state, err := m.GenerateOAuthState("google")
	require.NoError(t, err)
	require.NotEmpty(t, state)

	err = m.ParseOAuthState(state, "google")
	require.NoError(t, err)
}

func TestParseOAuthStateWrongProvider(t *testing.T) {
	m := newManager()
	state, err := m.GenerateOAuthState("google")
	require.NoError(t, err)

	err = m.ParseOAuthState(state, "github")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestParseOAuthStateExpired(t *testing.T) {
	now := time.Now()
	claims := OAuthStateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now.Add(-20 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-10 * time.Minute)),
			Issuer:    oauthStateIssuer,
		},
		Provider: "google",
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)

	m := newManager()
	err = m.ParseOAuthState(signed, "google")
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestGenerateAndValidateOAuth2FAChallenge(t *testing.T) {
	m := newManager()
	userID := uuid.New()

	challenge, err := m.GenerateOAuth2FAChallenge(userID)
	require.NoError(t, err)

	got, jti, expiresAt, err := m.ValidateOAuth2FAChallenge(challenge)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
	assert.NotEmpty(t, jti)
	assert.True(t, expiresAt.After(time.Now()))
}

func TestValidateOAuth2FAChallengeExpired(t *testing.T) {
	m := newManager()
	userID := uuid.New()
	now := time.Now()
	claims := OAuth2FAChallengeClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now.Add(-10 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-5 * time.Minute)),
			Issuer:    oauth2FAChallengeIssuer,
		},
		UserID: userID,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)

	_, _, _, err = m.ValidateOAuth2FAChallenge(signed)
	assert.ErrorIs(t, err, ErrTokenExpired)
}
