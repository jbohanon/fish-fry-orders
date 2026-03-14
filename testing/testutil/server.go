package testutil

import (
	"net/http"
	"net/http/httptest"

	"git.nonahob.net/jacob/fish-fry-orders/internal/api"
	"git.nonahob.net/jacob/fish-fry-orders/internal/auth"
	"git.nonahob.net/jacob/fish-fry-orders/internal/database"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// TestServer wraps an Echo server for testing
type TestServer struct {
	Server      *echo.Echo
	AuthService *auth.Service
	BaseURL     string
	httpServer  *httptest.Server
}

// NewTestServer creates a new test server with the provided repository
func NewTestServer(repo database.Repository) *TestServer {
	e := echo.New()

	// Initialize services
	authService := auth.NewService("test-worker-password", "test-admin-password")

	// Test allowed origins
	testAllowedOrigins := []string{"http://localhost:5173", "http://localhost:8080"}

	// Initialize API handlers
	orderHandler := api.NewOrderHandler(repo, testAllowedOrigins)
	menuHandler := api.NewMenuHandler(repo)
	sessionHandler := api.NewSessionHandler(repo, orderHandler)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Public routes
	authGroup := e.Group("/api/auth")
	authGroup.POST("/login", authService.HandleLogin)
	authGroup.POST("/logout", authService.HandleLogout)
	authGroup.GET("/check", authService.HandleCheck)

	// API routes (protected)
	apiGroup := e.Group("/api", authService.Middleware())

	// Session routes
	apiGroup.GET("/session", sessionHandler.GetCurrentSession)
	apiGroup.POST("/session", sessionHandler.CreateSession)
	apiGroup.PUT("/session/:id", sessionHandler.UpdateSession)
	apiGroup.POST("/session/:id/close", sessionHandler.CloseSession)
	apiGroup.GET("/sessions", sessionHandler.GetSessions)
	apiGroup.GET("/sessions/compare", sessionHandler.CompareSessions)
	apiGroup.GET("/sessions/:id", sessionHandler.GetSession)
	apiGroup.GET("/sessions/:id/orders", sessionHandler.GetSessionOrders)

	// Order routes
	apiGroup.POST("/orders", orderHandler.CreateOrder)
	apiGroup.GET("/orders", orderHandler.GetOrders)
	apiGroup.GET("/orders/:id", orderHandler.GetOrder)
	apiGroup.PUT("/orders/:id/status", orderHandler.UpdateOrderStatus)
	apiGroup.DELETE("/orders/purge", orderHandler.PurgeOrders)

	// Menu routes
	apiGroup.GET("/menu-items", menuHandler.GetMenuItems)
	apiGroup.GET("/menu-items/:id", menuHandler.GetMenuItem)
	apiGroup.POST("/menu-items", menuHandler.CreateMenuItem)
	apiGroup.PUT("/menu-items/:id", menuHandler.UpdateMenuItem)
	apiGroup.PUT("/menu-items/order", menuHandler.UpdateMenuItemsOrder)
	apiGroup.DELETE("/menu-items/:id", menuHandler.DeleteMenuItem)

	// WebSocket for real-time updates
	e.GET("/ws/orders", orderHandler.HandleWebSocket, authService.Middleware())

	// System routes
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Create HTTP test server
	httpServer := httptest.NewServer(e)

	return &TestServer{
		Server:      e,
		AuthService: authService,
		BaseURL:     httpServer.URL,
		httpServer:  httpServer,
	}
}

// Close shuts down the test server
func (ts *TestServer) Close() {
	if ts.httpServer != nil {
		ts.httpServer.Close()
	}
}

// AuthenticatedRequest creates an authenticated HTTP request
// Note: This method is not used in the current test suite, but kept for potential future use
func (ts *TestServer) AuthenticatedRequest(method, path, password string) (*http.Request, error) {
	req := httptest.NewRequest(method, ts.BaseURL+path, nil)
	
	// Login first to get session cookie
	loginReq := httptest.NewRequest("POST", ts.BaseURL+"/api/auth/login", nil)
	loginReq.Header.Set("Content-Type", "application/json")
	
	// For now, we'll use a simpler approach - create a session directly
	// In a real test, you'd make a login request and extract the cookie
	user, sessionID, err := ts.AuthService.Login(loginReq, httptest.NewRecorder(), password)
	if err != nil {
		return nil, err
	}
	_ = user // Use user to avoid unused variable
	
	// Set the session cookie
	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionID,
	})
	
	return req, nil
}
