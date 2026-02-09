package testing

import (
	"net/http"
	"testing"

	"git.nonahob.net/jacob/fish-fry-orders/testing/testutil"
)

func TestAuth_Login(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	tests := []struct {
		name           string
		password       string
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:           "login with worker password",
			password:       "test-worker-password",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var loginResp map[string]string
				testutil.ParseJSONResponse(t, resp, &loginResp)
				if loginResp["role"] != "worker" {
					t.Errorf("Expected role 'worker', got '%s'", loginResp["role"])
				}
				// Check for session cookie
				cookies := resp.Cookies()
				foundSession := false
				for _, cookie := range cookies {
					if cookie.Name == "session" {
						foundSession = true
						if cookie.Value == "" {
							t.Error("Expected session cookie to have a value")
						}
						break
					}
				}
				if !foundSession {
					t.Error("Expected session cookie to be set")
				}
			},
		},
		{
			name:           "login with admin password",
			password:       "test-admin-password",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var loginResp map[string]string
				testutil.ParseJSONResponse(t, resp, &loginResp)
				if loginResp["role"] != "admin" {
					t.Errorf("Expected role 'admin', got '%s'", loginResp["role"])
				}
			},
		},
		{
			name:           "login with invalid password",
			password:       "wrong-password",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "login with empty password",
			password:       "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]string{"password": tt.password}
			resp := testutil.UnauthenticatedRequest(t, server, "POST", "/api/auth/login", body)

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestAuth_Logout(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	t.Run("logout when authenticated", func(t *testing.T) {
		// First login
		loginBody := map[string]string{"password": "test-worker-password"}
		loginResp := testutil.UnauthenticatedRequest(t, server, "POST", "/api/auth/login", loginBody)
		testutil.AssertStatusCode(t, loginResp, http.StatusOK)

		// Extract session cookie
		var sessionCookie *http.Cookie
		for _, cookie := range loginResp.Cookies() {
			if cookie.Name == "session" {
				sessionCookie = cookie
				break
			}
		}

		if sessionCookie == nil {
			t.Fatal("No session cookie received from login")
		}

		// Now logout
		logoutBody := map[string]string{}
		logoutResp := testutil.UnauthenticatedRequest(t, server, "POST", "/api/auth/logout", logoutBody)
		// Logout redirects, so we expect 302 or 200
		if logoutResp.StatusCode != http.StatusFound && logoutResp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 302 or 200, got %d", logoutResp.StatusCode)
		}
	})
}

func TestAuth_Check(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	tests := []struct {
		name           string
		password       string
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:           "check authenticated user",
			password:       "test-worker-password",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var checkResp map[string]string
				testutil.ParseJSONResponse(t, resp, &checkResp)
				if checkResp["role"] != "worker" {
					t.Errorf("Expected role 'worker', got '%s'", checkResp["role"])
				}
			},
		},
		{
			name:           "check unauthenticated user",
			password:       "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "GET", "/api/auth/check", tt.password, nil)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "GET", "/api/auth/check", nil)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestAuth_ProtectedEndpoint(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	t.Run("access protected endpoint without auth", func(t *testing.T) {
		resp := testutil.UnauthenticatedRequest(t, server, "GET", "/api/orders", nil)
		// Should redirect to /auth
		if resp.StatusCode != http.StatusFound {
			t.Errorf("Expected status 302 (redirect), got %d", resp.StatusCode)
		}
	})

	t.Run("access protected endpoint with auth", func(t *testing.T) {
		resp := testutil.AuthenticatedRequest(t, server, "GET", "/api/orders", "test-worker-password", nil)
		// Should succeed
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
