package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Role represents a user role
type Role string

const (
	RoleWorker Role = "worker"
	RoleAdmin  Role = "admin"
)

// User represents an authenticated user
type User struct {
	Role Role
}

// Service handles authentication
type Service struct {
	workerPassword string
	adminPassword  string
	sessions       map[string]*User
}

// NewService creates a new auth service
func NewService(workerPassword, adminPassword string) *Service {
	return &Service{
		workerPassword: workerPassword,
		adminPassword:  adminPassword,
		sessions:       make(map[string]*User),
	}
}

// Login authenticates a user and sets a session cookie
// Returns the authenticated user and session ID on success
func (s *Service) Login(r *http.Request, w http.ResponseWriter, password string) (*User, string, error) {
	var user *User
	switch {
	case password == s.workerPassword:
		user = &User{Role: RoleWorker}
	case password == s.adminPassword:
		user = &User{Role: RoleAdmin}
	default:
		return nil, "", ErrInvalidPassword
	}

	// Create session
	sessionID := uuid.New().String()
	s.sessions[sessionID] = user

	// Determine if request is over HTTPS (directly or via proxy)
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
	})

	return user, sessionID, nil
}

// Logout removes the session cookie
func (s *Service) Logout(w http.ResponseWriter) {
	// Remove session cookie (set Secure=false to ensure it clears in both HTTP and HTTPS)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

// IsAuthenticated checks if the request has a valid session
func (s *Service) IsAuthenticated(r *http.Request) bool {
	_, err := s.GetUser(r)
	return err == nil
}

// GetUser returns the authenticated user
func (s *Service) GetUser(r *http.Request) (*User, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil, ErrUnauthorized
	}

	user, exists := s.sessions[cookie.Value]
	if !exists {
		return nil, ErrUnauthorized
	}

	return user, nil
}

// Authorize checks if the user has the required role
func (s *Service) Authorize(r *http.Request, requiredRole Role) error {
	user, err := s.GetUser(r)
	if err != nil {
		return err
	}

	if user.Role != requiredRole && user.Role != RoleAdmin {
		return ErrUnauthorized
	}

	return nil
}

// Errors
var (
	ErrInvalidPassword = &Error{Message: "invalid password"}
	ErrUnauthorized    = &Error{Message: "unauthorized"}
)

// Error represents an authentication error
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}
