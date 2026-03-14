package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"git.nonahob.net/jacob/fish-fry-orders/internal/api"
	"git.nonahob.net/jacob/fish-fry-orders/internal/auth"
	"git.nonahob.net/jacob/fish-fry-orders/internal/config"
	"git.nonahob.net/jacob/fish-fry-orders/internal/database"
	"git.nonahob.net/jacob/fish-fry-orders/internal/logger"
	"git.nonahob.net/jacob/fish-fry-orders/internal/metrics"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func getAllowedOrigins(cfg *config.Config) []string {
	origins := cfg.AllowedOrigins
	if len(origins) == 0 {
		// Fallback for local development
		origins = []string{"http://localhost:5173", "http://localhost:8080"}
	}
	return origins
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database pool/repository without requiring DB availability at startup.
	ctx := context.Background()
	dbPool, dbRepo, err := database.InitFromConfigNoPing(ctx, &cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbPool.Close()
	logger.Info(
		"Database pool initialized (startup is DB-tolerant)",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"dbname", cfg.Database.DBName,
	)

	// Create Echo instance
	e := echo.New()

	// Initialize services
	authService := auth.NewService(cfg.Auth.WorkerPassword, cfg.Auth.AdminPassword)

	// Get allowed origins for CORS and WebSocket
	allowedOrigins := getAllowedOrigins(cfg)
	logger.Info("Configured allowed origins", "origins", allowedOrigins)

	// Initialize API handlers
	orderHandler := api.NewOrderHandler(dbRepo, authService, allowedOrigins)
	menuHandler := api.NewMenuHandler(dbRepo)
	sessionHandler := api.NewSessionHandler(dbRepo, orderHandler)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	var dbUp atomic.Bool
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))
	e.Use(httpMetricsMiddleware())
	e.Use(errorLoggingMiddleware(&dbUp))

	// Public routes
	authGroup := e.Group("/api/auth")
	authGroup.POST("/login", authService.HandleLogin)
	authGroup.POST("/logout", authService.HandleLogout)
	authGroup.GET("/check", authService.HandleCheck)

	// API routes (protected - all authenticated users)
	apiGroup := e.Group("/api", authService.Middleware())
	apiGroup.POST("/orders", orderHandler.CreateOrder)
	apiGroup.GET("/orders", orderHandler.GetOrders)
	apiGroup.GET("/orders/:id", orderHandler.GetOrder)
	apiGroup.PUT("/orders/:id", orderHandler.UpdateOrder)
	apiGroup.PUT("/orders/:id/status", orderHandler.UpdateOrderStatus)
	apiGroup.GET("/menu-items", menuHandler.GetMenuItems)
	apiGroup.GET("/menu-items/:id", menuHandler.GetMenuItem)

	// Admin-only routes
	adminGroup := e.Group("/api", authService.Middleware(), authService.RequireRole(auth.RoleAdmin))
	adminGroup.POST("/menu-items", menuHandler.CreateMenuItem)
	adminGroup.PUT("/menu-items/:id", menuHandler.UpdateMenuItem)
	adminGroup.PUT("/menu-items/order", menuHandler.UpdateMenuItemsOrder)
	adminGroup.DELETE("/menu-items/:id", menuHandler.DeleteMenuItem)
	adminGroup.DELETE("/orders/purge", orderHandler.PurgeOrders)
	adminGroup.POST("/session", sessionHandler.CreateSession)
	adminGroup.PUT("/session/:id", sessionHandler.UpdateSession)
	adminGroup.POST("/session/:id/close", sessionHandler.CloseSession)

	// Session routes (read-only for all authenticated users)
	apiGroup.GET("/session", sessionHandler.GetCurrentSession)
	apiGroup.GET("/sessions", sessionHandler.GetSessions)
	apiGroup.GET("/sessions/:id", sessionHandler.GetSession)
	apiGroup.GET("/sessions/:id/orders", sessionHandler.GetSessionOrders)
	apiGroup.GET("/sessions/compare", sessionHandler.CompareSessions)

	// WebSocket for real-time updates
	e.GET("/ws/orders", orderHandler.HandleWebSocket, authService.Middleware())

	// System routes
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	e.GET("/health", func(c echo.Context) error {
		// Check database connectivity with timeout.
		pingCtx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if err := dbPool.Ping(pingCtx); err != nil {
			return c.String(http.StatusServiceUnavailable, "Database unavailable")
		}
		return c.String(http.StatusOK, "OK")
	})

	backgroundCtx, cancelBackground := context.WithCancel(context.Background())
	defer cancelBackground()

	// Keep a continuously updated database liveness metric for alerting.
	go monitorDatabase(backgroundCtx, dbPool, 15*time.Second, &dbUp)
	go runMigrationsWithRetry(backgroundCtx, &cfg.Database, 15*time.Second)

	// Start server
	logger.Info("Starting server", "address", cfg.HTTP.Address)
	go func() {
		if err := e.Start(cfg.HTTP.Address); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err.Error())
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")
	cancelBackground()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.ErrorWithErr("Error during shutdown", err)
		e.Logger.Fatal(err)
	}
	logger.Info("Server stopped")
}

type dbPinger interface {
	Ping(context.Context) error
}

func runMigrationsWithRetry(ctx context.Context, cfg *config.DatabaseConfig, interval time.Duration) {
	dbCfg := &database.Config{
		Host:     cfg.Host,
		Port:     fmt.Sprintf("%d", cfg.Port),
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		SSLMode:  cfg.SSLMode,
	}

	// Try once immediately, then periodically until success or shutdown.
	for {
		err := dbCfg.Migrate()
		if err == nil {
			logger.Info("Database migrations are up to date")
			return
		}

		logger.ErrorWithErr("Database migrations unavailable; will retry", err, "retry_interval", interval.String())

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func monitorDatabase(ctx context.Context, dbPool dbPinger, interval time.Duration, dbUp *atomic.Bool) {
	check := func() {
		start := time.Now()
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := dbPool.Ping(pingCtx)
		cancel()
		if dbUp != nil {
			dbUp.Store(err == nil)
		}
		metrics.RecordDatabasePing(err == nil, time.Since(start).Seconds())
		if err != nil {
			logger.ErrorWithErr("Database ping failed", err)
		}
	}

	check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

func httpMetricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}
			metrics.RecordHTTPRequest(
				c.Request().Method,
				path,
				c.Response().Status,
				time.Since(start).Seconds(),
			)
			return err
		}
	}
}

// errorLoggingMiddleware logs errors before they're returned to the client
func errorLoggingMiddleware(dbUp *atomic.Bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err != nil {
				if shouldReturnBadGateway(err, dbUp) {
					return echo.NewHTTPError(http.StatusBadGateway, "Database unavailable")
				}
				// Log the error with context
				he, ok := err.(*echo.HTTPError)
				if ok {
					logger.ErrorWithErr(
						"HTTP error",
						err,
						"method", c.Request().Method,
						"path", c.Path(),
						"status", he.Code,
						"message", he.Message,
						"remote_ip", c.RealIP(),
					)
				} else {
					logger.ErrorWithErr(
						"Handler error",
						err,
						"method", c.Request().Method,
						"path", c.Path(),
						"remote_ip", c.RealIP(),
					)
				}
			}
			return err
		}
	}
}

func shouldReturnBadGateway(err error, dbUp *atomic.Bool) bool {
	if dbUp == nil || dbUp.Load() {
		return false
	}

	var he *echo.HTTPError
	if !errors.As(err, &he) {
		return true
	}

	return he.Code >= http.StatusInternalServerError
}
