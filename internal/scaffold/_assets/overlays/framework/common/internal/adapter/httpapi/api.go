package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authdto "__GO_CRAFT_MODULE__/internal/adapter/http/dto/auth"
	userdto "__GO_CRAFT_MODULE__/internal/adapter/http/dto/user"
	appauth "__GO_CRAFT_MODULE__/internal/auth"
	"__GO_CRAFT_MODULE__/internal/user"
)

const timeLayout = "2006-01-02T15:04:05Z07:00"

type AuthService interface {
	Register(context.Context, *user.User) error
	Login(context.Context, string, string) (*appauth.Tokens, error)
	Refresh(context.Context, string) (*appauth.Tokens, error)
	Logout(context.Context, string) error
	VerifyEmail(context.Context, string) error
	ResendVerification(context.Context, string) error
	SendVerificationForUser(context.Context, int) error
	Me(context.Context, int) (*user.User, error)
}

type UserService interface {
	Create(context.Context, *user.User) error
	List(context.Context, int, int) ([]*user.User, int64, error)
	GetByID(context.Context, int) (*user.User, error)
	UpdateProfile(context.Context, int, string, string) error
	ChangePassword(context.Context, int, string, string) error
	ChangeRole(context.Context, int, int, string) error
	Delete(context.Context, int, int) error
	DeleteSelf(context.Context, int, string) error
}

type AccessTokenValidator interface {
	ValidateAccessToken(string) (appauth.Claims, error)
}

type API struct {
	users  UserService
	auth   AuthService
	tokens AccessTokenValidator
	logger *slog.Logger
	ready  func(context.Context) bool
}

func New(users UserService, auth AuthService, tokens AccessTokenValidator, logger *slog.Logger, ready func(context.Context) bool) *API {
	return &API{users: users, auth: auth, tokens: tokens, logger: logger, ready: ready}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", a.healthReady)
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/refresh", a.refresh)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("POST /api/auth/verify-email", a.verifyEmail)
	mux.HandleFunc("POST /api/auth/resend-verification", a.resendVerification)
	mux.HandleFunc("GET /api/auth/me", a.protected(a.me, false))
	mux.HandleFunc("PATCH /api/users/me", a.protected(a.updateSelf, false))
	mux.HandleFunc("PUT /api/users/me/password", a.protected(a.changePassword, false))
	mux.HandleFunc("DELETE /api/users/me", a.protected(a.deleteSelf, false))
	mux.HandleFunc("GET /api/users", a.protected(a.getUsers, true))
	mux.HandleFunc("POST /api/users", a.protected(a.createUser, true))
	mux.HandleFunc("GET /api/users/{id}", a.protected(a.getUserByID, true))
	mux.HandleFunc("PUT /api/users/{id}", a.protected(a.updateUser, true))
	mux.HandleFunc("PUT /api/users/{id}/role", a.protected(a.changeRole, true))
	mux.HandleFunc("DELETE /api/users/{id}", a.protected(a.deleteUser, true))

	return mux
}

type accountHandler func(http.ResponseWriter, *http.Request, *user.User)

func (a *API) protected(next accountHandler, admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			writeError(w, http.StatusUnauthorized, "invalid authorization header")
			return
		}

		claims, err := a.tokens.ValidateAccessToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		account, err := a.users.GetByID(r.Context(), claims.UserID)
		if err != nil || account == nil || account.TokenVersion != claims.TokenVersion || !account.EmailVerified() {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if admin && !account.IsAdmin() {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		next(w, r, account)
	}
}

func (a *API) healthReady(w http.ResponseWriter, r *http.Request) {
	if a.ready == nil || !a.ready(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req authdto.RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	account := user.User{Username: req.Username, Email: req.Email, Password: req.Password}
	if err := a.auth.Register(r.Context(), &account); err != nil {
		a.logError(r.Context(), "register failed", err)
		writeDomainError(w, err)

		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id": account.ID, "username": account.Username, "email": account.Email,
		"message": "check your email to verify your account",
	})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req authdto.LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	tokens, err := a.auth.Login(r.Context(), req.Email, req.Password)
	if errors.Is(err, appauth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if errors.Is(err, appauth.ErrEmailUnverified) {
		writeError(w, http.StatusForbidden, "email is not verified")
		return
	}

	if err != nil {
		a.logError(r.Context(), "login failed", err)
		writeDomainError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, authResponse(tokens))
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var req authdto.RefreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	tokens, err := a.auth.Refresh(r.Context(), req.RefreshToken)
	if errors.Is(err, appauth.ErrInvalidToken) || errors.Is(err, appauth.ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if err != nil {
		a.logError(r.Context(), "refresh failed", err)
		writeDomainError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, authResponse(tokens))
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	var req authdto.RefreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := a.auth.Logout(r.Context(), req.RefreshToken); err != nil {
		a.logError(r.Context(), "logout failed", err)
		writeDomainError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req authdto.VerifyEmailRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := a.auth.VerifyEmail(r.Context(), req.Token); err != nil {
		if errors.Is(err, appauth.ErrInvalidToken) {
			writeError(w, http.StatusBadRequest, "invalid or expired verification token")
			return
		}

		a.logError(r.Context(), "verify email failed", err)
		writeDomainError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "email verified"})
}

func (a *API) resendVerification(w http.ResponseWriter, r *http.Request) {
	var req authdto.ResendVerificationRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := a.auth.ResendVerification(r.Context(), req.Email); err != nil {
		a.logError(r.Context(), "resend verification failed", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "if the account requires verification, an email has been sent"})
}

func (*API) me(w http.ResponseWriter, _ *http.Request, account *user.User) {
	writeJSON(w, http.StatusOK, authdto.MeResponse{
		ID: account.ID, Username: account.Username, Email: account.Email, Role: string(account.Role),
		EmailVerified: account.EmailVerified(), PendingEmail: account.PendingEmail,
	})
}

func (a *API) getUsers(w http.ResponseWriter, r *http.Request, _ *user.User) {
	page, ok := positiveQuery(w, r, "page", user.DefaultPage)
	if !ok {
		return
	}

	limit, ok := positiveQuery(w, r, "limit", user.DefaultLimit)
	if !ok {
		return
	}

	if limit > user.MaxLimit {
		writeError(w, http.StatusBadRequest, "limit must not exceed 100")
		return
	}

	accounts, total, err := a.users.List(r.Context(), page, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	items := make([]userdto.UserSummary, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, userdto.UserSummary{ID: account.ID, Username: account.Username, Email: account.Email, Role: string(account.Role)})
	}

	writeJSON(w, http.StatusOK, userdto.UserListResponse{Data: items, Page: page, Limit: limit, Total: total})
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request, _ *user.User) {
	var req userdto.RegisterUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	account := user.User{Username: req.Username, Email: req.Email, Password: req.Password}
	if err := a.users.Create(r.Context(), &account); err != nil {
		writeDomainError(w, err)
		return
	}

	if err := a.auth.SendVerificationForUser(r.Context(), account.ID); err != nil {
		a.logger.WarnContext(r.Context(), "send verification failed", "user_id", account.ID, "error", err)
	}

	writeJSON(w, http.StatusCreated, userResponse(&account))
}

func (a *API) getUserByID(w http.ResponseWriter, r *http.Request, _ *user.User) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	account, err := a.users.GetByID(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse(account))
}

func (a *API) updateUser(w http.ResponseWriter, r *http.Request, _ *user.User) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	a.updateProfile(w, r, id)
}

func (a *API) updateSelf(w http.ResponseWriter, r *http.Request, account *user.User) {
	a.updateProfile(w, r, account.ID)
}

func (a *API) updateProfile(w http.ResponseWriter, r *http.Request, id int) {
	var req userdto.UpdateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	current, err := a.users.GetByID(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	username, email := current.Username, current.Email
	if req.Username != nil {
		username = *req.Username
	}

	if req.Email != nil {
		email = *req.Email
	}

	if err := a.users.UpdateProfile(r.Context(), id, username, email); err != nil {
		writeDomainError(w, err)
		return
	}

	if err := a.auth.SendVerificationForUser(r.Context(), id); err != nil {
		a.logger.WarnContext(r.Context(), "send verification failed", "user_id", id, "error", err)
	}

	updated, err := a.users.GetByID(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse(updated))
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request, account *user.User) {
	var req userdto.ChangePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := a.users.ChangePassword(r.Context(), account.ID, req.CurrentPassword, req.NewPassword); err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed; sign in again"})
}

func (a *API) changeRole(w http.ResponseWriter, r *http.Request, actor *user.User) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	var req userdto.ChangeRoleRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := a.users.ChangeRole(r.Context(), actor.ID, id, req.Role); err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "role changed"})
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request, actor *user.User) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	if err := a.users.Delete(r.Context(), actor.ID, id); err != nil {
		writeDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteSelf(w http.ResponseWriter, r *http.Request, account *user.User) {
	var req userdto.DeleteSelfRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := a.users.DeleteSelf(r.Context(), account.ID, req.CurrentPassword); err != nil {
		writeDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return false
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}

	return true
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, user.ErrInvalidInput), errors.Is(err, user.ErrInvalidRole):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, user.ErrInvalidPassword), errors.Is(err, appauth.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, user.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, appauth.ErrVerificationRate):
		writeError(w, http.StatusTooManyRequests, "verification requested too recently")
	case errors.Is(err, user.ErrNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, user.ErrDuplicateEmail), errors.Is(err, user.ErrDuplicateUsername), errors.Is(err, user.ErrLastAdmin):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func authResponse(tokens *appauth.Tokens) authdto.AuthResponse {
	return authdto.AuthResponse{
		AccessToken: tokens.AccessToken, AccessExpiresAt: tokens.AccessExpiresAt.Format(timeLayout),
		RefreshToken: tokens.RefreshToken, RefreshExpiresAt: tokens.RefreshExpiresAt.Format(timeLayout),
	}
}

func userResponse(account *user.User) userdto.UserResponse {
	return userdto.UserResponse{
		ID: account.ID, Username: account.Username, Email: account.Email, Role: string(account.Role),
		EmailVerified: account.EmailVerified(), PendingEmail: account.PendingEmail,
	}
}

func positiveQuery(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		writeError(w, http.StatusBadRequest, name+" must be a positive integer")
		return 0, false
	}

	return value, true
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	value, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || value <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return 0, false
	}

	return value, true
}

func (a *API) logError(ctx context.Context, message string, err error) {
	if a.logger != nil {
		a.logger.ErrorContext(ctx, message, "error", err)
	}
}
