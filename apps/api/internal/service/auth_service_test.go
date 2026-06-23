package service

import (
	"context"
	"encoding/base32"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthService(fs *testsupport.FakeStore) *AuthService {
	jm := auth.NewJWTManager("test-secret", time.Hour, 24*time.Hour)
	return NewAuthService(fs, jm, testsupport.NewTestLogger())
}

func hashPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	require.NoError(t, err)
	return string(h)
}

// currentTOTP computes the valid TOTP code for the given base32 secret.
func currentTOTP(t *testing.T, secret string) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	require.NoError(t, err)
	return generateTOTPCode(key, time.Now().Unix()/30)
}

func TestGetSetupStatus(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 3, nil }
	s := newAuthService(fs)
	st, err := s.GetSetupStatus(context.Background())
	require.NoError(t, err)
	assert.True(t, st.Initialized)
}

func TestGetSetupStatusError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 0, errors.New("db") }
	s := newAuthService(fs)
	_, err := s.GetSetupStatus(context.Background())
	require.Error(t, err)
}

func TestRegisterDisabledWhenUsersExist(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 1, nil }
	s := newAuthService(fs)
	_, err := s.Register(context.Background(), RegisterInput{})
	require.ErrorContains(t, err, "registration is disabled")
}

func TestRegisterEmailExists(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 0, nil }
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{}, nil
	}
	s := newAuthService(fs)
	_, err := s.Register(context.Background(), RegisterInput{Email: "a@b.com", Password: "password123"})
	require.ErrorContains(t, err, "already registered")
}

func TestRegisterOrgCreateError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 0, nil }
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.OrganizationsStore.CreateFn = func(ctx context.Context, org *model.Organization) error {
		return errors.New("org failed")
	}
	s := newAuthService(fs)
	_, err := s.Register(context.Background(), RegisterInput{Email: "a@b.com", Password: "password123", OrgName: "Acme"})
	require.Error(t, err)
}

func TestRegisterUserCreateError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 0, nil }
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.UsersStore.CreateFn = func(ctx context.Context, user *model.User) error {
		return errors.New("user failed")
	}
	s := newAuthService(fs)
	_, err := s.Register(context.Background(), RegisterInput{Email: "a@b.com", Password: "password123", OrgName: "Acme"})
	require.Error(t, err)
}

func TestRegisterSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 0, nil }
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.OrganizationsStore.CreateFn = func(ctx context.Context, org *model.Organization) error {
		org.ID = uuid.New()
		return nil
	}
	fs.UsersStore.CreateFn = func(ctx context.Context, user *model.User) error {
		user.ID = uuid.New()
		return nil
	}
	s := newAuthService(fs)
	res, err := s.Register(context.Background(), RegisterInput{Email: "a@b.com", Password: "password123", OrgName: "Acme", DisplayName: "A"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.Equal(t, model.RoleAdmin, res.User.Role)
	assert.True(t, res.User.IsSuperAdmin)
}

func TestLoginInvalidEmail(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	s := newAuthService(fs)
	_, err := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "x"})
	require.ErrorContains(t, err, "invalid credentials")
}

func TestLoginWrongPassword(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{PasswordHash: hashPassword(t, "correct")}, nil
	}
	s := newAuthService(fs)
	_, err := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "wrong"})
	require.ErrorContains(t, err, "invalid credentials")
}

func TestLoginRequires2FA(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{PasswordHash: hashPassword(t, "pw"), TwoFAEnabled: true}, nil
	}
	s := newAuthService(fs)
	res, err := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "pw"})
	require.NoError(t, err)
	assert.True(t, res.Requires2FA)
}

func TestLoginBad2FACode(t *testing.T) {
	secret := generateTOTPSecret()
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{PasswordHash: hashPassword(t, "pw"), TwoFAEnabled: true, TwoFASecret: secret}, nil
	}
	s := newAuthService(fs)
	_, err := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "pw", TwoFACode: "000000"})
	require.ErrorContains(t, err, "two-factor")
}

func TestLoginSuccessWith2FA(t *testing.T) {
	secret := generateTOTPSecret()
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: uuid.New()}, PasswordHash: hashPassword(t, "pw"), TwoFAEnabled: true, TwoFASecret: secret}, nil
	}
	s := newAuthService(fs)
	res, err := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "pw", TwoFACode: currentTOTP(t, secret)})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestLoginSuccessNo2FA(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: uuid.New()}, PasswordHash: hashPassword(t, "pw")}, nil
	}
	s := newAuthService(fs)
	res, err := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "pw"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestLoginBlockedGoogleOnly(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{PasswordHash: hashPassword(t, "pw")}, nil
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingAuthGoogleOnly {
			return "true", nil
		}
		return "", nil
	}
	s := newAuthService(fs)
	_, err := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "pw"})
	require.ErrorContains(t, err, "password sign-in is disabled")
}

func TestLoginGoogleOnlyNoPasswordOracle(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{PasswordHash: hashPassword(t, "pw")}, nil
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingAuthGoogleOnly {
			return "true", nil
		}
		return "", nil
	}
	s := newAuthService(fs)

	_, wrongErr := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "wrong"})
	require.ErrorContains(t, wrongErr, "password sign-in is disabled")

	_, rightErr := s.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "pw"})
	require.ErrorContains(t, rightErr, "password sign-in is disabled")
	assert.Equal(t, wrongErr.Error(), rightErr.Error())
}

func TestLoginSuperAdminBypassesGoogleOnly(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: uuid.New()},
			PasswordHash: hashPassword(t, "pw"),
			IsSuperAdmin: true,
		}, nil
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingAuthGoogleOnly {
			return "true", nil
		}
		return "", nil
	}
	s := newAuthService(fs)
	res, err := s.Login(context.Background(), LoginInput{Email: "admin@b.com", Password: "pw"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestLoginPasswordAllowedOutsideOAuthDomains(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{
			BaseModel:    model.BaseModel{ID: uuid.New()},
			PasswordHash: hashPassword(t, "pw"),
			Email:        "user@gmail.com",
		}, nil
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		switch key {
		case model.SettingAuthGoogleOnly:
			return "false", nil
		case model.SettingOAuthAllowedDomains:
			return "company.com", nil
		default:
			return "", nil
		}
	}
	s := newAuthService(fs)
	res, err := s.Login(context.Background(), LoginInput{Email: "user@gmail.com", Password: "pw"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestRefreshInvalidToken(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newAuthService(fs)
	_, err := s.Refresh(context.Background(), RefreshInput{RefreshToken: "garbage"})
	require.ErrorContains(t, err, "invalid refresh token")
}

func TestRefreshUserNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	jm := auth.NewJWTManager("test-secret", time.Hour, 24*time.Hour)
	pair, _ := jm.GenerateTokenPair(uuid.New(), uuid.New(), "admin", 0)
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("not found")
	}
	s := NewAuthService(fs, jm, testsupport.NewTestLogger())
	_, err := s.Refresh(context.Background(), RefreshInput{RefreshToken: pair.RefreshToken})
	require.ErrorContains(t, err, "user not found")
}

func TestRefreshTokenVersionMismatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	jm := auth.NewJWTManager("test-secret", time.Hour, 24*time.Hour)
	pair, _ := jm.GenerateTokenPair(uuid.New(), uuid.New(), "admin", 0)
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{TokenVersion: 5}, nil
	}
	s := NewAuthService(fs, jm, testsupport.NewTestLogger())
	_, err := s.Refresh(context.Background(), RefreshInput{RefreshToken: pair.RefreshToken})
	require.ErrorContains(t, err, "session invalidated")
}

func TestRefreshSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	jm := auth.NewJWTManager("test-secret", time.Hour, 24*time.Hour)
	userID := uuid.New()
	pair, _ := jm.GenerateTokenPair(userID, uuid.New(), "admin", 0)
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: userID}, TokenVersion: 0}, nil
	}
	s := NewAuthService(fs, jm, testsupport.NewTestLogger())
	res, err := s.Refresh(context.Background(), RefreshInput{RefreshToken: pair.RefreshToken})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestGetUser(t *testing.T) {
	fs := testsupport.NewFakeStore()
	uid := uuid.New()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: uid}}, nil
	}
	s := newAuthService(fs)
	u, err := s.GetUser(context.Background(), uid)
	require.NoError(t, err)
	assert.Equal(t, uid, u.ID)
}

func TestUpdateProfile(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{}, nil
	}
	s := newAuthService(fs)
	first, last, name, avatar := "F", "L", "Display", "bear"
	u, err := s.UpdateProfile(context.Background(), uuid.New(), UpdateProfileInput{
		FirstName: &first, LastName: &last, DisplayName: &name, AvatarURL: &avatar,
	})
	require.NoError(t, err)
	assert.Equal(t, "F", u.FirstName)
	assert.Equal(t, "bear", u.AvatarURL)
}

func TestUpdateProfileNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("missing")
	}
	s := newAuthService(fs)
	_, err := s.UpdateProfile(context.Background(), uuid.New(), UpdateProfileInput{})
	require.ErrorContains(t, err, "user not found")
}

func TestUpdateProfileEmptyDisplayName(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{}, nil
	}
	s := newAuthService(fs)
	empty := "  "
	_, err := s.UpdateProfile(context.Background(), uuid.New(), UpdateProfileInput{DisplayName: &empty})
	require.ErrorContains(t, err, "display name cannot be empty")
}

func TestUpdateProfileInvalidAvatar(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{}, nil
	}
	s := newAuthService(fs)
	bad := "dragon"
	_, err := s.UpdateProfile(context.Background(), uuid.New(), UpdateProfileInput{AvatarURL: &bad})
	require.ErrorContains(t, err, "invalid avatar")
}

func TestChangePassword(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{PasswordHash: hashPassword(t, "oldpw")}, nil
	}
	s := newAuthService(fs)
	err := s.ChangePassword(context.Background(), uuid.New(), ChangePasswordInput{CurrentPassword: "oldpw", NewPassword: "newpassword"})
	require.NoError(t, err)
}

func TestChangePasswordNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("missing")
	}
	s := newAuthService(fs)
	err := s.ChangePassword(context.Background(), uuid.New(), ChangePasswordInput{})
	require.ErrorContains(t, err, "user not found")
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{PasswordHash: hashPassword(t, "oldpw")}, nil
	}
	s := newAuthService(fs)
	err := s.ChangePassword(context.Background(), uuid.New(), ChangePasswordInput{CurrentPassword: "wrong", NewPassword: "newpassword"})
	require.ErrorContains(t, err, "current password is incorrect")
}

func TestSetup2FA(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{Email: "a@b.com"}, nil
	}
	s := newAuthService(fs)
	resp, err := s.Setup2FA(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Secret)
	assert.Contains(t, resp.QRCode, "otpauth://")
}

func TestSetup2FANotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("missing")
	}
	s := newAuthService(fs)
	_, err := s.Setup2FA(context.Background(), uuid.New())
	require.ErrorContains(t, err, "user not found")
}

func TestSetup2FAAlreadyEnabled(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{TwoFAEnabled: true}, nil
	}
	s := newAuthService(fs)
	_, err := s.Setup2FA(context.Background(), uuid.New())
	require.ErrorContains(t, err, "already enabled")
}

func TestVerify2FA(t *testing.T) {
	secret := generateTOTPSecret()
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{TwoFASecret: secret}, nil
	}
	s := newAuthService(fs)
	require.NoError(t, s.Verify2FA(context.Background(), uuid.New(), currentTOTP(t, secret)))
}

func TestVerify2FANotSetup(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{}, nil
	}
	s := newAuthService(fs)
	err := s.Verify2FA(context.Background(), uuid.New(), "123456")
	require.ErrorContains(t, err, "not been set up")
}

func TestVerify2FAInvalidCode(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{TwoFASecret: generateTOTPSecret()}, nil
	}
	s := newAuthService(fs)
	err := s.Verify2FA(context.Background(), uuid.New(), "000000")
	require.ErrorContains(t, err, "invalid 2FA code")
}

func TestVerify2FANotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("missing")
	}
	s := newAuthService(fs)
	require.ErrorContains(t, s.Verify2FA(context.Background(), uuid.New(), "123456"), "user not found")
}

func TestDisable2FA(t *testing.T) {
	secret := generateTOTPSecret()
	var (
		updated2FA    bool
		clearedSecret bool
		bumped        bool
	)
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{TwoFAEnabled: true, TwoFASecret: secret, TokenVersion: 2}, nil
	}
	fs.UsersStore.Update2FAFn = func(ctx context.Context, userID uuid.UUID, enabled bool, secret string) error {
		updated2FA = !enabled
		clearedSecret = secret == ""
		return nil
	}
	fs.UsersStore.BumpTokenVersionFn = func(ctx context.Context, id uuid.UUID) error {
		bumped = true
		return nil
	}
	s := newAuthService(fs)
	require.NoError(t, s.Disable2FA(context.Background(), uuid.New(), currentTOTP(t, secret)))
	assert.True(t, updated2FA)
	assert.True(t, clearedSecret)
	assert.True(t, bumped)
}

func TestLogout(t *testing.T) {
	userID := uuid.New()
	var bumped uuid.UUID
	fs := testsupport.NewFakeStore()
	fs.UsersStore.BumpTokenVersionFn = func(ctx context.Context, id uuid.UUID) error {
		bumped = id
		return nil
	}
	s := newAuthService(fs)
	require.NoError(t, s.Logout(context.Background(), userID))
	assert.Equal(t, userID, bumped)
}

func TestLogoutDBError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.BumpTokenVersionFn = func(ctx context.Context, id uuid.UUID) error {
		return errors.New("connection refused")
	}
	s := newAuthService(fs)
	require.ErrorContains(t, s.Logout(context.Background(), uuid.New()), "connection refused")
}

func TestDisable2FANotEnabled(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{}, nil
	}
	s := newAuthService(fs)
	require.ErrorContains(t, s.Disable2FA(context.Background(), uuid.New(), "123456"), "not enabled")
}

func TestDisable2FAInvalidCode(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{TwoFAEnabled: true, TwoFASecret: generateTOTPSecret()}, nil
	}
	s := newAuthService(fs)
	require.ErrorContains(t, s.Disable2FA(context.Background(), uuid.New(), "000000"), "invalid 2FA code")
}

func TestDisable2FANotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("missing")
	}
	s := newAuthService(fs)
	require.ErrorContains(t, s.Disable2FA(context.Background(), uuid.New(), "123456"), "user not found")
}

func TestListAvatars(t *testing.T) {
	s := newAuthService(testsupport.NewFakeStore())
	assert.NotEmpty(t, s.ListAvatars())
}

func TestVerifyTOTPInvalidSecret(t *testing.T) {
	assert.False(t, verifyTOTP("!!!notbase32!!!", "123456"))
}
