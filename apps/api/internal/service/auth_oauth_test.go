package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oauthauth "github.com/orkai-dev/orkai/apps/api/internal/auth/oauth"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
)

func TestLoginWithOAuthEmailNotVerified(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newAuthService(fs)

	_, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-1",
		Email:         "user@company.com",
		EmailVerified: false,
	})
	require.ErrorIs(t, err, ErrOAuthEmailNotVerified)
}

func TestLoginWithOAuthDomainNotAllowed(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingOAuthAllowedDomains {
			return "company.com", nil
		}
		return "", nil
	}
	s := newAuthService(fs)

	_, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-1",
		Email:         "user@gmail.com",
		EmailVerified: true,
	})
	require.ErrorIs(t, err, ErrOAuthDomainNotAllowed)
}

func TestLoginWithOAuthFailsClosedOnSettingsError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	settingsErr := errors.New("db timeout")
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return "", settingsErr
	}
	// A user record exists, so the only thing standing between this login and a
	// token is the allowlist check — which must fail closed when it can't be read.
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: uuid.New()}, Email: email}, nil
	}
	s := newAuthService(fs)

	_, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-1",
		Email:         "user@gmail.com",
		EmailVerified: true,
	})
	require.ErrorIs(t, err, settingsErr)
}

func TestLoginWithOAuthDomainAllowed(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingOAuthAllowedDomains {
			return "company.com", nil
		}
		return "", nil
	}
	fs.IdentitiesStore.GetByProviderSubjectFn = func(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
		return nil, errors.New("not found")
	}
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: userID}, OrgID: orgID, Email: email}, nil
	}
	fs.IdentitiesStore.CreateFn = func(ctx context.Context, identity *model.UserIdentity) error {
		return nil
	}
	s := newAuthService(fs)

	res, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-1",
		Email:         "user@company.com",
		EmailVerified: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestLoginWithOAuthNoAccount(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.IdentitiesStore.GetByProviderSubjectFn = func(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
		return nil, errors.New("not found")
	}
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	s := newAuthService(fs)

	_, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-1",
		Email:         "missing@company.com",
		EmailVerified: true,
	})
	require.ErrorIs(t, err, ErrOAuthNoAccount)
}

func TestLoginWithOAuthExistingIdentityLink(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.IdentitiesStore.GetByProviderSubjectFn = func(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
		return &model.UserIdentity{UserID: userID, Provider: provider, Subject: subject}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: userID}, OrgID: orgID, Email: "user@company.com"}, nil
	}
	s := newAuthService(fs)

	res, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-linked",
		Email:         "user@company.com",
		EmailVerified: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestEmailDomainAllowed(t *testing.T) {
	assert.True(t, emailDomainAllowed("a@company.com", ""))
	assert.True(t, emailDomainAllowed("a@company.com", "company.com,other.com"))
	assert.False(t, emailDomainAllowed("a@gmail.com", "company.com"))
}

func TestLoginWithOAuthCreatesIdentityLink(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	var created *model.UserIdentity
	fs := testsupport.NewFakeStore()
	fs.IdentitiesStore.GetByProviderSubjectFn = func(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
		return nil, errors.New("not found")
	}
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: userID}, OrgID: orgID, Email: email}, nil
	}
	fs.IdentitiesStore.CreateFn = func(ctx context.Context, identity *model.UserIdentity) error {
		created = identity
		return nil
	}
	s := newAuthService(fs)

	_, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-new",
		Email:         "user@company.com",
		EmailVerified: true,
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, userID, created.UserID)
	assert.Equal(t, oauthauth.GoogleProviderName, created.Provider)
	assert.Equal(t, "sub-new", created.Subject)
}

func TestLoginWithOAuthIdentityLinkMissingUser(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.IdentitiesStore.GetByProviderSubjectFn = func(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
		return &model.UserIdentity{UserID: uuid.New()}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("not found")
	}
	s := newAuthService(fs)

	_, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-orphan",
		Email:         "user@company.com",
		EmailVerified: true,
	})
	require.ErrorIs(t, err, ErrOAuthNoAccount)
}

func TestLoginWithOAuthTokenGeneration(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.IdentitiesStore.GetByProviderSubjectFn = func(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
		return &model.UserIdentity{UserID: userID}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: userID},
			OrgID:        orgID,
			Email:        "user@company.com",
			Role:         model.RoleAdmin,
			TokenVersion: 2,
		}, nil
	}
	s := newAuthService(fs)

	res, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-1",
		Email:         "user@company.com",
		EmailVerified: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.RefreshToken)
	assert.True(t, res.ExpiresAt.After(time.Now()))
}

func TestLoginWithOAuthRequires2FA(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.IdentitiesStore.GetByProviderSubjectFn = func(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
		return &model.UserIdentity{UserID: userID}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: userID},
			OrgID:        orgID,
			Email:        "user@company.com",
			Role:         model.RoleAdmin,
			TwoFAEnabled: true,
			TwoFASecret:  generateTOTPSecret(),
			TokenVersion: 1,
		}, nil
	}
	s := newAuthService(fs)

	res, err := s.LoginWithOAuth(context.Background(), oauthauth.GoogleProviderName, &oauthauth.UserIdentity{
		Subject:       "sub-1",
		Email:         "user@company.com",
		EmailVerified: true,
	})
	require.NoError(t, err)
	assert.True(t, res.Requires2FA)
	assert.NotEmpty(t, res.OAuth2FAChallenge)
	assert.Empty(t, res.AccessToken)
	assert.Empty(t, res.RefreshToken)
}

func TestCompleteOAuth2FA(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	secret := generateTOTPSecret()
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: userID},
			OrgID:        orgID,
			Email:        "user@company.com",
			Role:         model.RoleMember,
			TwoFAEnabled: true,
			TwoFASecret:  secret,
			TokenVersion: 0,
		}, nil
	}
	s := newAuthService(fs)

	challenge, err := s.jwtManager.GenerateOAuth2FAChallenge(userID)
	require.NoError(t, err)

	res, err := s.CompleteOAuth2FA(context.Background(), challenge, currentTOTP(t, secret))
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
}

func TestCompleteOAuth2FARejectsReplay(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	secret := generateTOTPSecret()
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: userID},
			OrgID:        orgID,
			Email:        "user@company.com",
			Role:         model.RoleMember,
			TwoFAEnabled: true,
			TwoFASecret:  secret,
		}, nil
	}
	s := newAuthService(fs)

	challenge, err := s.jwtManager.GenerateOAuth2FAChallenge(userID)
	require.NoError(t, err)

	code := currentTOTP(t, secret)
	res, err := s.CompleteOAuth2FA(context.Background(), challenge, code)
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)

	// Replaying the same challenge token must be rejected even within its validity window.
	_, err = s.CompleteOAuth2FA(context.Background(), challenge, code)
	require.ErrorContains(t, err, "invalid or expired OAuth challenge")
}

func TestCompleteOAuth2FAWrongCodeConsumesChallenge(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	secret := generateTOTPSecret()
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: userID},
			OrgID:        orgID,
			Email:        "user@company.com",
			Role:         model.RoleMember,
			TwoFAEnabled: true,
			TwoFASecret:  secret,
		}, nil
	}
	s := newAuthService(fs)

	challenge, err := s.jwtManager.GenerateOAuth2FAChallenge(userID)
	require.NoError(t, err)

	// A wrong code must burn the challenge so it cannot be reused to keep guessing.
	_, err = s.CompleteOAuth2FA(context.Background(), challenge, "000000")
	require.ErrorContains(t, err, "invalid two-factor authentication code")

	// Even the correct code is now rejected: the single guess has been spent.
	_, err = s.CompleteOAuth2FA(context.Background(), challenge, currentTOTP(t, secret))
	require.ErrorContains(t, err, "invalid or expired OAuth challenge")
}

func TestCompleteOAuth2FAInvalidCode(t *testing.T) {
	userID := uuid.New()
	secret := generateTOTPSecret()
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: userID},
			TwoFAEnabled: true,
			TwoFASecret:  secret,
		}, nil
	}
	s := newAuthService(fs)

	challenge, err := s.jwtManager.GenerateOAuth2FAChallenge(userID)
	require.NoError(t, err)

	_, err = s.CompleteOAuth2FA(context.Background(), challenge, "000000")
	require.ErrorContains(t, err, "invalid two-factor authentication code")
}

func TestCompleteOAuth2FAExpiredChallenge(t *testing.T) {
	s := newAuthService(testsupport.NewFakeStore())
	_, err := s.CompleteOAuth2FA(context.Background(), "not-a-valid-challenge", "123456")
	require.ErrorContains(t, err, "invalid or expired OAuth challenge")
}
