package testutil

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"

	"github.com/jbohanon/fish-fry-orders-v2/internal/api"
	"github.com/jbohanon/fish-fry-orders-v2/internal/auth"
	"github.com/jbohanon/fish-fry-orders-v2/internal/database"
	"github.com/jbohanon/fish-fry-orders-v2/internal/ui"
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

	// Initialize API handlers
	orderHandler := api.NewOrderHandler(repo)
	menuHandler := api.NewMenuHandler(repo)
	chatHandler := api.NewChatHandler(repo)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Get template directory
	templateDir := filepath.Join("ui", "templates")
	templateService, err := ui.NewTemplateService(templateDir)
	if err != nil {
		// If templates don't exist, that's okay for API tests
		_ = templateService
	}

	// Public routes
	authGroup := e.Group("/api/auth")
	authGroup.POST("/login", authService.HandleLogin)
	authGroup.POST("/logout", authService.HandleLogout)
	authGroup.GET("/check", authService.HandleCheck)

	// API routes (protected)
	apiGroup := e.Group("/api", authService.Middleware())
	apiGroup.POST("/orders", orderHandler.CreateOrder)
	apiGroup.GET("/orders", orderHandler.GetOrders)
	apiGroup.GET("/orders/:id", orderHandler.GetOrder)
	apiGroup.PUT("/orders/:id/status", orderHandler.UpdateOrderStatus)
	apiGroup.GET("/menu-items", menuHandler.GetMenuItems)
	apiGroup.GET("/menu-items/:id", menuHandler.GetMenuItem)
	apiGroup.POST("/menu-items", menuHandler.CreateMenuItem)
	apiGroup.PUT("/menu-items/:id", menuHandler.UpdateMenuItem)
	apiGroup.PUT("/menu-items/order", menuHandler.UpdateMenuItemsOrder)
	apiGroup.DELETE("/menu-items/:id", menuHandler.DeleteMenuItem)
	apiGroup.DELETE("/orders/purge", orderHandler.PurgeOrders)
	apiGroup.POST("/orders/:id/messages", chatHandler.CreateMessage)
	apiGroup.GET("/orders/:id/messages", chatHandler.GetMessages)

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
