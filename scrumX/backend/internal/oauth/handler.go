package oauth

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	service     *Service
	redisClient *redis.Client
}

func NewHandler(service *Service, redisClient *redis.Client) *Handler {
	return &Handler{service: service, redisClient: redisClient}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/oauth/authorize", h.authorizePage)
	app.Post("/oauth/authorize", h.authorizeLogin)
	app.Post("/oauth/authorize/consent", h.authorizeConsent)
	app.Post("/oauth/token", middleware.RateLimitMiddleware(10, time.Minute, h.redisClient), h.token)
	app.Post("/oauth/revoke", h.revoke)
	app.Post("/oauth/introspect", h.introspect)
	app.Get("/oauth/userinfo", h.userinfo)
}

type authorizeLoginForm struct {
	ClientID            string `form:"client_id"`
	RedirectURI         string `form:"redirect_uri"`
	ResponseType        string `form:"response_type"`
	Scope               string `form:"scope"`
	State               string `form:"state"`
	CodeChallenge       string `form:"code_challenge"`
	CodeChallengeMethod string `form:"code_challenge_method"`
	Email               string `form:"email"`
	Password            string `form:"password"`
}

type consentForm struct {
	Session string `form:"session"`
	OrgID   string `form:"org_id"`
	Action  string `form:"action"`
}

type tokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	Code         string `json:"code" form:"code"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

type revokeRequest struct {
	Token        string `json:"token" form:"token"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
}

type introspectRequest struct {
	Token        string `json:"token" form:"token"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
}

func (h *Handler) authorizePage(c *fiber.Ctx) error {
	req := AuthorizeRequest{
		ClientID:            c.Query("client_id"),
		RedirectURI:         c.Query("redirect_uri"),
		ResponseType:        c.Query("response_type"),
		Scope:               c.Query("scope"),
		State:               c.Query("state"),
		CodeChallenge:       c.Query("code_challenge"),
		CodeChallengeMethod: c.Query("code_challenge_method"),
	}
	client, scopes, err := h.service.ValidateAuthorizeRequest(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderErrorPage(err.Error()))
	}
	return c.Type("html").SendString(renderLoginPage(client, req, scopes, ""))
}

func (h *Handler) authorizeLogin(c *fiber.Ctx) error {
	var form authorizeLoginForm
	if err := c.BodyParser(&form); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderErrorPage("invalid form submission"))
	}
	req := AuthorizeRequest{
		ClientID:            form.ClientID,
		RedirectURI:         form.RedirectURI,
		ResponseType:        form.ResponseType,
		Scope:               form.Scope,
		State:               form.State,
		CodeChallenge:       form.CodeChallenge,
		CodeChallengeMethod: form.CodeChallengeMethod,
	}
	client, scopes, err := h.service.ValidateAuthorizeRequest(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderErrorPage(err.Error()))
	}
	userID, _, err := h.service.AuthenticateUser(c.Context(), form.Email, form.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).Type("html").SendString(renderLoginPage(client, req, scopes, err.Error()))
	}
	orgs, err := h.service.ListUserOrganizations(c.Context(), userID)
	if err != nil || len(orgs) == 0 {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderLoginPage(client, req, scopes, "no organizations available for this user"))
	}
	session, err := h.service.CreateAuthorizeSession(c.Context(), userID, req, scopes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Type("html").SendString(renderErrorPage(err.Error()))
	}
	return c.Type("html").SendString(renderConsentPage(client, orgs, scopes, session))
}

func (h *Handler) authorizeConsent(c *fiber.Ctx) error {
	var form consentForm
	if err := c.BodyParser(&form); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderErrorPage("invalid consent submission"))
	}
	approved := strings.EqualFold(strings.TrimSpace(form.Action), "approve")
	claims, err := h.service.consumeAuthorizeSession(c.Context(), form.Session, approved)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderErrorPage(err.Error()))
	}
	redirectURI := strings.TrimSpace(claims.RedirectURI)
	state := strings.TrimSpace(claims.State)
	if !approved {
		return c.Redirect(withQuery(redirectURI, map[string]string{"error": "access_denied", "state": state}), http.StatusFound)
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderErrorPage("invalid authorization session user"))
	}
	orgID, err := uuid.Parse(strings.TrimSpace(form.OrgID))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderErrorPage("valid organization selection is required"))
	}
	if _, err := h.service.ResolveUserOrg(c.Context(), userID, orgID); err != nil {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderErrorPage(err.Error()))
	}
	code, err := h.service.CreateAuthorizationCode(c.Context(), userID, orgID, claims)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Type("html").SendString(renderErrorPage(err.Error()))
	}
	return c.Redirect(withQuery(redirectURI, map[string]string{"code": code, "state": state}), http.StatusFound)
}

func (h *Handler) token(c *fiber.Ctx) error {
	var req tokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_request"})
	}
	switch strings.TrimSpace(req.GrantType) {
	case "authorization_code":
		resp, err := h.service.ExchangeAuthorizationCode(c.Context(), AuthorizationCodeGrant{
			GrantType:    req.GrantType,
			Code:         req.Code,
			RedirectURI:  req.RedirectURI,
			ClientID:     req.ClientID,
			ClientSecret: req.ClientSecret,
			CodeVerifier: req.CodeVerifier,
		})
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(resp)
	case "refresh_token":
		resp, err := h.service.RefreshAccessToken(c.Context(), RefreshTokenGrant{
			GrantType:    req.GrantType,
			RefreshToken: req.RefreshToken,
			ClientID:     req.ClientID,
			ClientSecret: req.ClientSecret,
		})
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(resp)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported_grant_type"})
	}
}

func (h *Handler) revoke(c *fiber.Ctx) error {
	var req revokeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_request"})
	}
	if err := h.service.RevokeToken(c.Context(), req.ClientID, req.ClientSecret, req.Token); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusOK)
}

func (h *Handler) introspect(c *fiber.Ctx) error {
	var req introspectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_request"})
	}
	payload, err := h.service.IntrospectToken(c.Context(), req.ClientID, req.ClientSecret, req.Token)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(payload)
}

func (h *Handler) userinfo(c *fiber.Ctx) error {
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
	}
	payload, err := h.service.UserInfo(c.Context(), parts[1])
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(payload)
}

func renderLoginPage(client Client, req AuthorizeRequest, scopes []string, errMsg string) string {
	return mustRender(loginTemplate, map[string]any{
		"Client":  client,
		"Request": req,
		"Scopes":  strings.Join(scopes, " "),
		"Error":   errMsg,
	})
}

func renderConsentPage(client Client, orgs []Organization, scopes []string, session string) string {
	return mustRender(consentTemplate, map[string]any{
		"Client":  client,
		"Scopes":  scopes,
		"Session": session,
		"Orgs":    orgs,
	})
}

func renderErrorPage(message string) string {
	return mustRender(errorTemplate, map[string]any{"Message": message})
}

func mustRender(raw string, data any) string {
	tmpl := template.Must(template.New("page").Parse(raw))
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, data)
	return buf.String()
}

func withQuery(base string, values map[string]string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := parsed.Query()
	for key, value := range values {
		q.Set(key, value)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

const loginTemplate = `<!doctype html>
<html>
<head><meta charset="utf-8"><title>Authorize {{.Client.Name}}</title></head>
<body style="font-family: sans-serif; max-width: 520px; margin: 40px auto; line-height: 1.5;">
  <h1>Authorize {{.Client.Name}}</h1>
  <p>{{.Client.Name}} is requesting access with these scopes:</p>
  <p><code>{{.Scopes}}</code></p>
  {{if .Error}}<p style="color: #b91c1c;">{{.Error}}</p>{{end}}
  <form method="post" action="/oauth/authorize">
    <input type="hidden" name="client_id" value="{{.Request.ClientID}}">
    <input type="hidden" name="redirect_uri" value="{{.Request.RedirectURI}}">
    <input type="hidden" name="response_type" value="{{.Request.ResponseType}}">
    <input type="hidden" name="scope" value="{{.Request.Scope}}">
    <input type="hidden" name="state" value="{{.Request.State}}">
    <input type="hidden" name="code_challenge" value="{{.Request.CodeChallenge}}">
    <input type="hidden" name="code_challenge_method" value="{{.Request.CodeChallengeMethod}}">
    <label>Email<br><input type="email" name="email" style="width: 100%; padding: 8px;"></label><br><br>
    <label>Password<br><input type="password" name="password" style="width: 100%; padding: 8px;"></label><br><br>
    <button type="submit" style="padding: 10px 16px;">Continue</button>
  </form>
</body>
</html>`

const consentTemplate = `<!doctype html>
<html>
<head><meta charset="utf-8"><title>Consent for {{.Client.Name}}</title></head>
<body style="font-family: sans-serif; max-width: 520px; margin: 40px auto; line-height: 1.5;">
  <h1>Consent required</h1>
  <p><strong>{{.Client.Name}}</strong> wants access to:</p>
  <ul>{{range .Scopes}}<li>{{.}}</li>{{end}}</ul>
  <form method="post" action="/oauth/authorize/consent">
    <input type="hidden" name="session" value="{{.Session}}">
    <label>Authorize for organization<br>
      <select name="org_id" style="width: 100%; padding: 8px;">
        {{range .Orgs}}<option value="{{.ID}}">{{.Name}} ({{.Role}})</option>{{end}}
      </select>
    </label><br><br>
    <button type="submit" name="action" value="approve" style="padding: 10px 16px;">Approve</button>
    <button type="submit" name="action" value="deny" style="padding: 10px 16px;">Deny</button>
  </form>
</body>
</html>`

const errorTemplate = `<!doctype html>
<html>
<head><meta charset="utf-8"><title>OAuth error</title></head>
<body style="font-family: sans-serif; max-width: 520px; margin: 40px auto; line-height: 1.5;">
  <h1>OAuth error</h1>
  <p>{{.Message}}</p>
</body>
</html>`
