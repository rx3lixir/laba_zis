package user

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/pkg/httputil"
	"github.com/rx3lixir/laba_zis/pkg/password"
)

type Handler struct {
	store       Store
	authService *auth.Service
	log         *slog.Logger
	dbTimeout   time.Duration
}

func NewHandler(store Store, authService *auth.Service, log *slog.Logger, dbTimeout time.Duration) *Handler {
	if dbTimeout == 0 {
		dbTimeout = 5 * time.Second
	}
	return &Handler{store, authService, log, dbTimeout}
}

func (h *Handler) RegisterUserRoutes(r chi.Router) {
	r.Get("/", httputil.Handler(h.HandleGetAllUsers, h.log))
	r.Get("/{id}", httputil.Handler(h.HandleGetUserByID, h.log))
	r.Get("/email/{email}", httputil.Handler(h.HandleGetUserByEmail, h.log))
	r.Delete("/{id}", httputil.Handler(h.HandleDeleteUser, h.log))
	r.Get("/me", httputil.Handler(h.HandleMe, h.log))
}

func (h *Handler) RegisterAuthRoutes(r chi.Router) {
	r.Post("/signup", httputil.Handler(h.HandleSignup, h.log))
	r.Post("/signin", httputil.Handler(h.HandleSignin, h.log))
	r.Post("/refresh", httputil.Handler(h.HandleRefreshToken, h.log))
}

func (h *Handler) dbCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), h.dbTimeout)
}

// HandleMe returns the currently authenticated user's profile.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.log.Debug("me endpoint accessed without authentication")
		return httputil.Unauthorized("User ID is invalid")
	}

	h.log.Debug("get current user request",
		"user_id", userID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		h.log.Error("failed to retrieve current user from database",
			"user_id", userID,
			"error", err)
		return httputil.NotFound("User not found")
	}

	response := map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleCreateUser - creates a new user
func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) error {
	req := new(CreateUserRequest)
	if err := httputil.DecodeJSON(r, req); err != nil {
		return err
	}

	h.log.Debug("create user request received",
		"email", req.Email,
		"username", req.Username)

	if err := validateCreateUserRequest(req); err != nil {
		h.log.Debug("user validation failed",
			"email", req.Email,
			"error", err)
		return httputil.BadRequest("Validation failed", map[string]string{
			"validation_error": err.Error(),
		})
	}

	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		h.log.Error("failed to hash password",
			"error", err)
		return httputil.Internal(err)
	}

	newUser := &User{
		Username: req.Username,
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Password: string(hashedPassword),
	}

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	if err := h.store.CreateUser(ctx, newUser); err != nil {
		h.log.Error("failed to create user in database",
			"email", newUser.Email,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("user created successfully",
		"user_id", newUser.ID,
		"email", newUser.Email,
		"username", newUser.Username)

	response := CreateUserResponse{
		ID:        newUser.ID,
		Username:  newUser.Username,
		Email:     newUser.Email,
		CreatedAt: newUser.CreatedAt,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleGetUserByID retrieves a user by their UUID.
func (h *Handler) HandleGetUserByID(w http.ResponseWriter, r *http.Request) error {
	userID, err := httputil.ParseUUID(r, "id")
	if err != nil {
		return err
	}

	h.log.Debug("get user by ID request",
		"user_id", userID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		h.log.Debug("user not found",
			"user_id", userID,
			"error", err)
		return httputil.NotFound("User not found")
	}

	response := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleGetAllUsers returns a paginated list of users.
func (h *Handler) HandleGetAllUsers(w http.ResponseWriter, r *http.Request) error {
	limit := 10
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 100 {
				limit = 100
			}
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	h.log.Debug("get all users request",
		"limit", limit,
		"offset", offset)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	users, err := h.store.GetAllUsers(ctx, limit, offset)
	if err != nil {
		h.log.Error("failed to retrieve users from database",
			"error", err)
		return httputil.Internal(err)
	}

	// Convert to response format
	userResponses := make([]UserResponse, 0, len(users))
	for _, user := range users {
		userResponses = append(userResponses, UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	h.log.Debug("users retrieved",
		"count", len(users))

	response := GetAllUsersResponse{
		Users:      userResponses,
		TotalCount: len(userResponses),
		Limit:      limit,
		Offset:     offset,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleGetUserByEmail retrieves a user by their email address (case-insensitive).
func (h *Handler) HandleGetUserByEmail(w http.ResponseWriter, r *http.Request) error {
	email := chi.URLParam(r, "email")
	if email == "" {
		return httputil.BadRequest("email is required")
	}

	h.log.Debug("get user by email request",
		"email", email)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	user, err := h.store.GetUserByEmail(ctx, email)
	if err != nil {
		h.log.Debug("user not found by email",
			"email", email,
			"error", err)
		return httputil.NotFound("User not found")
	}

	response := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleDeleteUser permanently removes a user by their UUID.
func (h *Handler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) error {
	userID, err := httputil.ParseUUID(r, "id")
	if err != nil {
		return err
	}

	h.log.Debug("delete user request",
		"user_id", userID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	if err := h.store.DeleteUser(ctx, userID); err != nil {
		h.log.Error("failed to delete user from database",
			"user_id", userID,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("user deleted successfully",
		"user_id", userID)

	response := DeleteUserResponse{
		Message: "User deleted successfully",
		ID:      userID,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleSignup creates a new user account and immediately returns access + refresh JWT tokens.
func (h *Handler) HandleSignup(w http.ResponseWriter, r *http.Request) error {
	req := new(SignupRequest)
	if err := httputil.DecodeJSON(r, req); err != nil {
		return err
	}

	h.log.Debug("signup request received",
		"email", req.Email,
		"username", req.Username)

	// Validate request
	err := validateCreateUserRequest(
		&CreateUserRequest{
			Username: req.Username,
			Email:    req.Email,
			Password: req.Password,
		},
	)
	if err != nil {
		h.log.Debug("signup validation failed",
			"email", req.Email,
			"error", err)
		return httputil.BadRequest("Validation failed", map[string]string{
			"validation_error": err.Error(),
		})
	}

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Check if user exists
	email := strings.ToLower(strings.TrimSpace(req.Email))

	userExists, err := h.store.ExistsByEmail(ctx, email)
	if err != nil {
		h.log.Error("failed to check existing user",
			"email", email,
			"error", err)
		return httputil.Internal(err)
	}
	if userExists {
		h.log.Warn("signup blocked - email already exists",
			"email", email)
		return httputil.BadRequest("User with this email already exists")
	}

	// Hash password
	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		h.log.Error("failed to hash password during signup",
			"error", err)
		return httputil.Internal(err)
	}

	newUser := &User{
		Username: req.Username,
		Email:    email,
		Password: string(hashedPassword),
	}

	if err := h.store.CreateUser(ctx, newUser); err != nil {
		h.log.Error("failed to create user during signup",
			"email", email,
			"error", err)
		return httputil.Internal(err)
	}

	// Generate tokens
	accessToken, err := h.authService.GenerateAccessToken(newUser.ID, newUser.Email, newUser.Username)
	if err != nil {
		h.log.Error("failed to generate access token",
			"user_id", newUser.ID,
			"error", err)
		return httputil.Internal(err)
	}

	refreshToken, err := h.authService.GenerateRefreshToken(newUser.ID)
	if err != nil {
		h.log.Error("failed to generate refresh token",
			"user_id", newUser.ID,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("user signed up successfully",
		"user_id", newUser.ID,
		"email", newUser.Email,
		"username", newUser.Username)

	response := SignupResponse{
		User: UserResponse{
			ID:        newUser.ID,
			Username:  newUser.Username,
			Email:     newUser.Email,
			CreatedAt: newUser.CreatedAt,
			UpdatedAt: newUser.UpdatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleSignin authenticates a user and returns JWT pair of tokens
func (h *Handler) HandleSignin(w http.ResponseWriter, r *http.Request) error {
	req := new(SigninRequest)
	if err := httputil.DecodeJSON(r, req); err != nil {
		return err
	}

	h.log.Debug("signin request received",
		"email", req.Email)

	if req.Email == "" {
		return httputil.BadRequest("Email is required")
	}
	if req.Password == "" {
		return httputil.BadRequest("Password is required")
	}

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := h.store.GetUserByEmail(ctx, email)
	if err != nil {
		h.log.Warn("signin failed - user not found",
			"email", email)
		return httputil.Unauthorized("Invalid email or password")
	}

	if !password.Verify(req.Password, user.Password) {
		h.log.Warn("signin failed - invalid password",
			"email", email,
			"user_id", user.ID)
		return httputil.Unauthorized("Invalid email or password")
	}

	// Generate tokens
	accessToken, err := h.authService.GenerateAccessToken(user.ID, user.Email, user.Username)
	if err != nil {
		h.log.Error("failed to generate access token",
			"user_id", user.ID,
			"error", err)
		return httputil.Internal(err)
	}

	refreshToken, err := h.authService.GenerateRefreshToken(user.ID)
	if err != nil {
		h.log.Error("failed to generate refresh token",
			"user_id", user.ID,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("user signed in successfully",
		"user_id", user.ID,
		"email", user.Email)

	response := SigninResponse{
		User: UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleRefreshToken generates new tokens using a refresh token
func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) error {
	req := new(RefreshTokenRequest)
	if err := httputil.DecodeJSON(r, req); err != nil {
		return err
	}

	h.log.Debug("token refresh request received")

	if req.RefreshToken == "" {
		return httputil.BadRequest("Refresh token is required")
	}

	userID, err := h.authService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		h.log.Warn("token refresh failed - invalid token",
			"error", err)
		return httputil.Unauthorized("Invalid or expired refresh token")
	}

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		h.log.Error("token refresh failed - user not found",
			"user_id", userID,
			"error", err)
		return httputil.NotFound("User not found")
	}

	newAccessToken, err := h.authService.GenerateAccessToken(userID, user.Email, user.Username)
	if err != nil {
		h.log.Error("failed to generate new access token",
			"user_id", userID,
			"error", err)
		return httputil.Internal(err)
	}

	newRefreshToken, err := h.authService.GenerateRefreshToken(userID)
	if err != nil {
		h.log.Error("failed to generate new refresh token",
			"user_id", userID,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("tokens refreshed successfully",
		"user_id", user.ID)

	response := SigninResponse{
		User: UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}
