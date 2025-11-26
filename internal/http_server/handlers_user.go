package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/storage/main_db"
	"github.com/rx3lixir/laba_zis/pkg/password"
)

// Handles creating a new user
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	req := new(CreateUserRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	s.log.Info(
		"recieved request",
		"handler", "HandleAddUser",
		"email", req.Email,
	)

	// Request validation
	if err := validateCreateUserRequest(req); err != nil {
		s.handleError(w, err)

		s.log.Error(
			"User validation failed",
			"user_email", req.Email,
			"error", err,
		)
		return
	}

	// Password hashing
	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		s.log.Error("Failed to hash password", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to proccess password")
		return
	}

	// Creating new user
	newUser := &maindb.User{
		Username: req.Username,
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Password: string(hashedPassword),
	}

	// Saving user to database
	if err := s.userStore.CreateUser(r.Context(), newUser); err != nil {
		s.log.Error("Failed to create user", "error", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to add user to database")
		return
	}

	// Building a response
	response := CreateUserResponse{
		ID:        newUser.ID,
		Username:  newUser.Username,
		Email:     newUser.Email,
		CreatedAt: newUser.CreatedAt,
	}

	s.log.Info(
		"User created successfully",
		"user_email", newUser.Email,
		"user_id", newUser.ID,
	)

	s.respondJSON(w, http.StatusCreated, response)
}

// Handles getting user using it's ID
func (s *Server) handleGetUserByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		s.respondError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	s.log.Info("Received request",
		"handler", "HandleGetUserByID",
		"id", id,
	)

	// Getting user from database
	user, err := s.userStore.GetUserByID(r.Context(), userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	// Building a response
	response := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	s.log.Info(
		"User created successfully",
		"user_email", user.Email,
		"user_id", user.ID,
	)

	// Writing a response
	s.respondJSON(w, http.StatusOK, response)
}

// Handles getting all users from database
func (s *Server) handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	s.log.Info("Recieved request", "handler", "HandleGetAllUsers")

	limitQuery := r.URL.Query().Get("limit")
	offsetQuery := r.URL.Query().Get("offset")

	// Default values
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

	// Get users from database
	users, err := s.userStore.GetUsers(r.Context(), limit, offset)
	if err != nil {
		s.handleError(w, err)
		return
	}

	s.log.Info("Got users", "count", len(users))

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

	s.log.Info("Users retrieved successfully", "count", len(users))

	// Writing a response
	s.respondJSON(w, http.StatusOK, response)
}

// Handles getting user by it's email
func (s *Server) handleGetUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")
	if email == "" {
		s.respondError(w, http.StatusBadRequest, "Email is required")
		return
	}

	s.log.Info("Recieved request",
		"handler", "HandleGetUsersByEmail",
		"email", email,
	)

	// Getting user from database
	user, err := s.userStore.GetUserByEmail(r.Context(), email)
	if err != nil {
		s.handleError(w, err)
		return
	}

	// Building a response
	response := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	// Writing a response
	s.respondJSON(w, http.StatusOK, response)
}

// Handles deleting user from database
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		s.respondError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Parsing uuid
	userID, err := uuid.Parse(id)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	s.log.Debug("Received request",
		"handler", "HandleDeleteUser",
		"id", userID,
	)

	if err := s.userStore.DeleteUser(r.Context(), userID); err != nil {
		s.handleError(w, err)
		return
	}

	response := DeleteUserResponse{
		Message: "User deleted successfully",
		ID:      userID,
	}

	s.log.Debug("User deleted successfully", "user_id", userID)
	s.respondJSON(w, http.StatusOK, response)
}
