package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	maindb "github.com/rx3lixir/laba_zis/internal/storage/main_db"
	"github.com/rx3lixir/laba_zis/pkg/password"
)

// HandleSignup registers a new user and returns JWT tokens
func (s *Server) HandleSignup(w http.ResponseWriter, r *http.Request) {
	req := new(SignupRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	s.log.Info(
		"Signup attempt",
		"handler", "HandleSignup",
		"email", req.Email,
	)

	if err := validateCreateUserRequest(&CreateUserRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	}); err != nil {
		s.handleError(w, err)
		s.log.Error("Signup validation failed", "email", req.Email, "error", err)
		return
	}

	userExist, _ := s.userStore.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if userExist != nil {
		s.respondError(w, http.StatusConflict, "User with this email already exists")
		return
	}

	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		s.log.Error("Failed to hash password", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	newUser := &maindb.User{
		Username: req.Username,
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Password: string(hashedPassword),
	}

	if err := s.userStore.CreateUser(r.Context(), newUser); err != nil {
		s.log.Error("Failed to create user", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	accessToken, err := s.jwtService.GenerateAccessToken(newUser.ID, newUser.Email, newUser.Username)
	if err != nil {
		s.log.Error("Failed to generate access token", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to generate pair of tokens")
		return
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(newUser.ID)
	if err != nil {
		s.log.Error("Failed to generate refresh token", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to generate tokens")
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

	s.log.Info(
		"User signed up successfully",
		"user_id", newUser.ID,
		"email", newUser.Email,
	)

	s.respondJSON(w, http.StatusCreated, response)
}

// HandleSignin authenticates a user and returns JWT pair of tokens
func (s *Server) HandleSignin(w http.ResponseWriter, r *http.Request) {
	req := new(SigninRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	s.log.Info(
		"Signin attempt",
		"handler", "HandleSignin",
		"email", req.Email,
	)

	if req.Email == "" {
		s.respondError(w, http.StatusBadRequest, "Email is required")
		return
	}
	if req.Password == "" {
		s.respondError(w, http.StatusBadRequest, "Password is required")
		return
	}

	user, err := s.userStore.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		s.log.Warn("Signin failed - user not found", "email", req.Email)
		s.respondError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if !password.Verify(req.Password, user.Password) {
		s.log.Warn("Signin failed - password is invalid", "email", req.Email)
		s.respondError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.Email, user.Username)
	if err != nil {
		s.log.Error("Failed to generate access token", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to generate tokens")
		return
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID)
	if err != nil {
		s.log.Error("Failed to generate refresh token", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to generate tokens")
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

	s.log.Info("User signed in successfully", "user_id", user.ID, "email", user.Email)
	s.respondJSON(w, http.StatusOK, response)
}

// HandleRefreshToken generates new tokens using a refresh token
func (s *Server) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	req := new(RefreshTokenRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	s.log.Info("Token refresh attempt", "handler", "HandleRefreshToken")

	if req.RefreshToken == "" {
		s.respondError(w, http.StatusBadRequest, "Refresh token is required")
		return
	}

	// Validate refresh token
	userID, err := s.jwtService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		s.log.Warn("Invalid refresh token", "error", err)
		s.respondError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	user, err := s.userStore.GetUserByID(r.Context(), userID)
	if err != nil {
		s.log.Error("Failed to get user during token refresh operation", "user_id", userID, "error", err)
		s.respondError(w, http.StatusUnauthorized, "User not found")
		return
	}

	newAccessToken, err := s.jwtService.GenerateAccessToken(userID, user.Email, user.Username)
	if err != nil {
		s.log.Error("Failed to generate new access token", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to generate tokens")
		return
	}

	newRefreshToken, err := s.jwtService.GenerateRefreshToken(userID)
	if err != nil {
		s.log.Error("Failed to generate new refresh token", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to generate tokens")
		return
	}

	response := RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
	}

	s.log.Info("Tokens refreshed successfully", "user_id", user.ID)
	s.respondJSON(w, http.StatusOK, response)
}
