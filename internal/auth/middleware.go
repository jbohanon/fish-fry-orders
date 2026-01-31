package auth

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// Middleware handles authentication for HTTP requests
func (s *Service) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip auth check for static files and auth endpoints
			path := c.Request().URL.Path
			if strings.HasPrefix(path, "/static") || path == "/auth" || path == "/api/auth/login" || path == "/api/auth/logout" || path == "/api/auth/check" {
				return next(c)
			}

			// Check authentication
			if !s.IsAuthenticated(c.Request()) {
				return c.Redirect(http.StatusFound, "/auth")
			}

			return next(c)
		}
	}
}

// RequireRole middleware checks for specific role access
func (s *Service) RequireRole(role Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if err := s.Authorize(c.Request(), role); err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			return next(c)
		}
	}
}
