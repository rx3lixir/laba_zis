package user

import (
	"context"
	"encoding/json"
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
}

func NewHandler(store Store, authService *auth.Service, log *slog.Logger) *Handler {
	return &Handler{
		store:       store,
		authService: authService,
		log:         log,
	}
}

func (h *Handler) RegisterUserRoutes(r chi.Router) {
	r.Get("/", httputil.Handler(h.HandleGetAllUsers))
	r.Get("/{id}", h.HandleGetUserByID)
	r.Get("/email/{email}", h.HandleGetUserByEmail)
	r.Delete("/{id}", h.HandleDeleteUser)
	r.Get("/me", h.HandleMe)
}

func (h *Handler) RegisterAuthRoutes(r chi.Router) {
	r.Post("/signup", h.HandleSignup)
	r.Post("/signin", h.HandleSignin)
	r.Post("/refresh", h.HandleRefreshToken)
}

func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "ID is invalid"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(
		map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	req := new(CreateUserRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})

		return
	}

	h.log.Debug(
		"recieved request",
		"handler", "HandleAddUser",
		"email", req.Email,
	)

	if err := validateCreateUserRequest(req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to validate user"})

		h.log.Error(
			"User validation failed",
			"user_email", req.Email,
			"error", err,
		)

		return
	}

	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to proccess password"})

		h.log.Error("Failed to hash passowrd", "error", err)

		return
	}

	newUser := &User{
		Username: req.Username,
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Password: string(hashedPassword),
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	if err := h.store.CreateUser(ctx, newUser); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})

		h.log.Error("Failed to create user", "error", err)

		return
	}

	response := CreateUserResponse{
		ID:        newUser.ID,
		Username:  newUser.Username,
		Email:     newUser.Email,
		CreatedAt: newUser.CreatedAt,
	}

	h.log.Debug(
		"User created",
		"user_email", newUser.Email,
		"user_id", newUser.ID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Handles getting user using it's ID
func (h *Handler) HandleGetUserByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User ID required"})

		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user ID format"})

		return
	}

	h.log.Debug(
		"Received request",
		"handler", "HandleGetUserByID",
		"id", id,
	)

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to retrieve user"})

		return
	}

	response := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	h.log.Debug(
		"User created successfully",
		"user_email", user.Email,
		"user_id", user.ID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Handles getting all users from database
func (h *Handler) HandleGetAllUsers(w http.ResponseWriter, r *http.Request) error {
	limitQuery := r.URL.Query().Get("limit")
	offsetQuery := r.URL.Query().Get("offset")

	limit := 10
	offset := 0

	if limitQuery != "" {
		if parsedLimit, err := strconv.Atoi(limitQuery); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			// To prevent abuse
			if limit > 100 {
				limit = 100
			}
		}
	}

	if offsetQuery != "" {
		if parsedOffset, err := strconv.Atoi(offsetQuery); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	users, err := h.store.GetAllUsers(ctx, limit, offset)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to retrieve users"})

		h.log.Error("Failed to retrieve users", "error", err)

		return
	}

	h.log.Debug(
		"Got users",
		"count",
		len(users),
	)

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

	response := GetAllUsersResponse{
		Users:      userResponses,
		TotalCount: len(userResponses),
		Limit:      limit,
		Offset:     offset,
	}

	h.log.Debug(
		"Users retrieved successfully",
		"count",
		len(users),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Handles getting user by it's email
func (h *Handler) HandleGetUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")
	if email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User email is required"})

		return
	}

	h.log.Debug(
		"Recieved request",
		"handler", "HandleGetUsersByEmail",
		"email", email,
	)

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	user, err := h.store.GetUserByEmail(ctx, email)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Falied to retrieve user"})

		return
	}

	h.log.Debug(
		"Retrieved user from database",
		"username", user.Username,
		"email", user.Email,
	)

	response := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Handles deleting user from database
func (h *Handler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User ID is required"})

		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid User ID format"})

		return
	}

	h.log.Debug(
		"Received request",
		"handler", "HandleDeleteUser",
		"id", userID,
	)

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	if err := h.store.DeleteUser(ctx, userID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete user"})

		return
	}

	response := DeleteUserResponse{
		Message: "User deleted successfully",
		ID:      userID,
	}

	h.log.Debug(
		"User deleted successfully",
		"user_id",
		userID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleSignup registers a new user and returns JWT tokens
func (h *Handler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	req := new(SignupRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})

		return
	}

	h.log.Debug(
		"Signup attempt",
		"handler", "HandleSignup",
		"email", req.Email,
	)

	err := validateCreateUserRequest(
		&CreateUserRequest{
			Username: req.Username,
			Email:    req.Email,
			Password: req.Password,
		},
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to validate request"})

		h.log.Error(
			"Signup validation failed",
			"email", req.Email,
			"error", err,
		)

		return
	}

	userExist, _ := h.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if userExist != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User with this email is already exists"})

		return
	}

	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to process password"})

		h.log.Error("Failed to hash password", "error", err)

		return
	}

	newUser := &User{
		Username: req.Username,
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Password: string(hashedPassword),
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	if err := h.store.CreateUser(ctx, newUser); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})

		h.log.Error("Failed to create user", "error", err)

		return
	}

	accessToken, err := h.authService.GenerateAccessToken(newUser.ID, newUser.Email, newUser.Username)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate tokens"})

		h.log.Error("Failed to generate tokens", "error", err)

		return
	}

	refreshToken, err := h.authService.GenerateRefreshToken(newUser.ID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate tokens"})

		h.log.Error("Failed to generate refresh token", "error", err)

		return
	}

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

	h.log.Debug(
		"User signed up successfully",
		"user_id", newUser.ID,
		"email", newUser.Email,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleSignin authenticates a user and returns JWT pair of tokens
func (h *Handler) HandleSignin(w http.ResponseWriter, r *http.Request) {
	req := new(SigninRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})

		return
	}

	h.log.Debug(
		"Signin attempt",
		"handler", "HandleSignin",
		"email", req.Email,
	)

	if req.Email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email is required"})

		return
	}
	if req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Passowrd is required"})

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	user, err := h.store.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User does not exists"})

		h.log.Warn("Signin failed - user not found", "email", req.Email)

		return
	}

	if !password.Verify(req.Password, user.Password) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email or passowrd is invalid"})

		h.log.Warn("Signin failed - password is invalid", "email", req.Email)

		return
	}

	accessToken, err := h.authService.GenerateAccessToken(user.ID, user.Email, user.Username)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate tokens"})

		h.log.Error("Failed to generate access token", "error", err)

		return
	}

	refreshToken, err := h.authService.GenerateRefreshToken(user.ID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate tokens"})

		h.log.Error("Failed to generate refresh token", "error", err)

		return
	}

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

	h.log.Debug(
		"User signed in successfully",
		"user_id", user.ID,
		"email", user.Email,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleRefreshToken generates new tokens using a refresh token
func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	req := new(RefreshTokenRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})

		return
	}

	h.log.Debug(
		"Token refresh attempt",
		"handler", "HandleRefreshToken",
	)

	if req.RefreshToken == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Refresh token is required"})

		return
	}

	userID, err := h.authService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired refresh token"})

		h.log.Debug("Invalid or expired refresh token", "error", err)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})

		h.log.Error("Failed to get user during token refresh operation", "user_id", userID, "error", err)

		return
	}

	newAccessToken, err := h.authService.GenerateAccessToken(userID, user.Email, user.Username)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate tokens"})

		h.log.Error("Failed to generate new access token", "error", err)

		return
	}

	newRefreshToken, err := h.authService.GenerateRefreshToken(userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate tokens"})

		h.log.Error("Failed to generate new refresh token", "error", err)

		return
	}

	// Frontend expect this
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

	h.log.Debug(
		"Tokens refreshed successfully",
		"user_id", user.ID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
