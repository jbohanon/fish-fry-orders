package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AuthenticatedRequest makes an authenticated HTTP request to the test server
func AuthenticatedRequest(t *testing.T, server *TestServer, method, path, password string, body interface{}) *http.Response {
	t.Helper()

	// Login first to get session cookie
	loginBody := map[string]string{"password": password}
	loginBodyBytes, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", server.BaseURL+"/api/auth/login", bytes.NewReader(loginBodyBytes))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	
	server.Server.ServeHTTP(loginRecorder, loginReq)
	
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("Failed to login: status %d, body: %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	// Extract session cookie from login response
	var sessionCookie *http.Cookie
	cookies := loginRecorder.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("No session cookie received from login")
	}

	// Create the actual request
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := httptest.NewRequest(method, server.BaseURL+path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(sessionCookie)

	recorder := httptest.NewRecorder()
	server.Server.ServeHTTP(recorder, req)
	return recorder.Result()
}

// UnauthenticatedRequest makes an unauthenticated HTTP request
func UnauthenticatedRequest(t *testing.T, server *TestServer, method, path string, body interface{}) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := httptest.NewRequest(method, server.BaseURL+path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	server.Server.ServeHTTP(recorder, req)
	return recorder.Result()
}

// ParseJSONResponse parses a JSON response body
func ParseJSONResponse(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("Failed to parse JSON response: %v, body: %s", err, string(body))
	}
}

// AssertStatusCode asserts that the response has the expected status code
func AssertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()

	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
	}
}
