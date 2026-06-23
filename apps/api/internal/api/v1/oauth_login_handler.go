package v1

import (
	"errors"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	oauthauth "github.com/orkai-dev/orkai/apps/api/internal/auth/oauth"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ProblemDetail
)

type OAuthLoginHandler struct {
	store    store.Store
	authSvc  *service.AuthService
	jwt      *auth.JWTManager
	registry *oauthauth.Registry
	appURL   string
	logger   *slog.Logger
}

func NewOAuthLoginHandler(
	s store.Store,
	authSvc *service.AuthService,
	jwt *auth.JWTManager,
	appURL string,
	logger *slog.Logger,
) *OAuthLoginHandler {
	return &OAuthLoginHandler{
		store:    s,
		authSvc:  authSvc,
		jwt:      jwt,
		registry: oauthauth.NewRegistry(s.Settings()),
		appURL:   appURL,
		logger:   logger,
	}
}

// Providers godoc
// @Summary      List OAuth login providers
// @Tags         oauth
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} apidocs.ProblemDetail
// @Router       /auth/providers [get]
func (h *OAuthLoginHandler) Providers(c *gin.Context) {
	ctx := c.Request.Context()
	providers := h.registry.ListProviders(ctx)
	googleOnly, err := h.store.Settings().Get(ctx, model.SettingAuthGoogleOnly)
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail("failed to load auth settings"))
		return
	}
	c.JSON(200, gin.H{
		"google":           providers["google"],
		"password_enabled": googleOnly != "true",
	})
}

// Login godoc
// @Summary      Start OAuth login flow
// @Tags         oauth
// @Param        provider path string true "OAuth provider name"
// @Success      302
// @Router       /auth/oauth/{provider}/login [get]
func (h *OAuthLoginHandler) Login(c *gin.Context) {
	providerName := c.Param("provider")
	provider, err := h.registry.GetProvider(c.Request.Context(), providerName)
	if err != nil {
		h.redirectToLogin(c, oauthErrorCode(err))
		return
	}

	state, err := h.jwt.GenerateOAuthState(providerName)
	if err != nil {
		h.redirectToLogin(c, "oauth_failed")
		return
	}

	redirectURI := h.appURL + "/api/v1/auth/oauth/" + providerName + "/callback"
	c.Redirect(302, provider.AuthCodeURL(state, redirectURI))
}

// Callback godoc
// @Summary      OAuth login callback
// @Tags         oauth
// @Param        provider path string true "OAuth provider name"
// @Param        code query string false "Authorization code"
// @Param        state query string false "CSRF state"
// @Success      302
// @Router       /auth/oauth/{provider}/callback [get]
func (h *OAuthLoginHandler) Callback(c *gin.Context) {
	providerName := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		h.redirectToLogin(c, "missing_code")
		return
	}

	if err := h.jwt.ParseOAuthState(state, providerName); err != nil {
		h.redirectToLogin(c, "invalid_state")
		return
	}

	provider, err := h.registry.GetProvider(c.Request.Context(), providerName)
	if err != nil {
		h.redirectToLogin(c, oauthErrorCode(err))
		return
	}

	redirectURI := h.appURL + "/api/v1/auth/oauth/" + providerName + "/callback"
	identity, err := provider.Exchange(c.Request.Context(), code, redirectURI)
	if err != nil {
		h.logger.Error("oauth token exchange failed", slog.String("provider", providerName), slog.Any("error", err))
		h.redirectToLogin(c, "token_exchange_failed")
		return
	}

	result, err := h.authSvc.LoginWithOAuth(c.Request.Context(), providerName, identity)
	if err != nil {
		h.redirectToLogin(c, oauthLoginErrorCode(err))
		return
	}

	if result.Requires2FA {
		fragment := url.Values{}
		fragment.Set("requires_2fa", "1")
		fragment.Set("challenge", result.OAuth2FAChallenge)
		c.Redirect(302, h.appURL+"/auth/callback#"+fragment.Encode())
		return
	}

	fragment := url.Values{}
	fragment.Set("access_token", result.AccessToken)
	fragment.Set("refresh_token", result.RefreshToken)
	c.Redirect(302, h.appURL+"/auth/callback#"+fragment.Encode())
}

type oauth2FAInput struct {
	Challenge string `json:"challenge" binding:"required"`
	Code      string `json:"code" binding:"required,len=6"`
}

// Complete2FA godoc
// @Summary      Complete OAuth login with 2FA
// @Tags         oauth
// @Accept       json
// @Produce      json
// @Param        body body v1.oauth2FAInput true "Request body"
// @Success      200 {object} apidocs.AuthResult
// @Failure      422 {object} apidocs.ProblemDetail
// @Router       /auth/oauth/2fa [post]
func (h *OAuthLoginHandler) Complete2FA(c *gin.Context) {
	var input oauth2FAInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	result, err := h.authSvc.CompleteOAuth2FA(c.Request.Context(), input.Challenge, input.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.ErrUnauthorized.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, result)
}

func (h *OAuthLoginHandler) redirectToLogin(c *gin.Context, code string) {
	c.Redirect(302, h.appURL+"/auth/login?error="+url.QueryEscape(code))
}

func oauthErrorCode(err error) string {
	switch {
	case errors.Is(err, oauthauth.ErrProviderDisabled):
		return "oauth_disabled"
	case errors.Is(err, oauthauth.ErrProviderNotConfigured):
		return "oauth_not_configured"
	case errors.Is(err, oauthauth.ErrProviderUnknown):
		return "unknown_provider"
	default:
		return "oauth_failed"
	}
}

func oauthLoginErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrOAuthEmailNotVerified):
		return "email_not_verified"
	case errors.Is(err, service.ErrOAuthDomainNotAllowed):
		return "domain_not_allowed"
	case errors.Is(err, service.ErrOAuthNoAccount):
		return "no_account"
	default:
		return "oauth_failed"
	}
}
