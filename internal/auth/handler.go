package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type LoginRequest struct {
	Password string `json:"password"`
}

type LoginResponse struct {
	Role string `json:"role"`
}

func (s *Service) HandleLogin(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	user, _, err := s.Login(c.Request(), c.Response(), req.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid password")
	}

	return c.JSON(http.StatusOK, LoginResponse{
		Role: string(user.Role),
	})
}

func (s *Service) HandleLogout(c echo.Context) error {
	s.Logout(c.Response())
	return c.JSON(http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Service) HandleCheck(c echo.Context) error {
	user, err := s.GetUser(c.Request())
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	return c.JSON(http.StatusOK, LoginResponse{
		Role: string(user.Role),
	})
}
