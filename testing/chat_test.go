package testing

import (
	"net/http"
	"strconv"
	"testing"

	"git.nonahob.net/jacob/fish-fry-orders/internal/api"
	"git.nonahob.net/jacob/fish-fry-orders/testing/testutil"
)

func TestChat_CreateMessage(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	testOrder := setup.TestData.Orders[0]

	tests := []struct {
		name           string
		password       string
		orderID        int
		request        api.CreateMessageRequest
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:     "create message for existing order",
			password:  "test-worker-password",
			orderID:   testOrder.ID,
			request: api.CreateMessageRequest{
				Content: "Test message content",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var message api.ChatMessageResponse
				testutil.ParseJSONResponse(t, resp, &message)
				if message.ID == "" {
					t.Error("Expected message ID to be set")
				}
				if message.OrderID != testOrder.ID {
					t.Errorf("Expected order ID %d, got %d", testOrder.ID, message.OrderID)
				}
				if message.Content != "Test message content" {
					t.Errorf("Expected content 'Test message content', got '%s'", message.Content)
				}
				if message.SenderRole == "" {
					t.Error("Expected sender role to be set")
				}
			},
		},
		{
			name:     "create message with empty content",
			password: "test-worker-password",
			orderID:  testOrder.ID,
			request: api.CreateMessageRequest{
				Content: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "create message for non-existent order",
			password: "test-worker-password",
			orderID:  99999,
			request: api.CreateMessageRequest{
				Content: "Test message",
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:     "create message unauthenticated",
			password: "",
			orderID:  testOrder.ID,
			request: api.CreateMessageRequest{
				Content: "Test message",
			},
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/orders/" + strconv.Itoa(tt.orderID) + "/messages"
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "POST", path, tt.password, tt.request)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "POST", path, tt.request)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestChat_GetMessages(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	testOrder := setup.TestData.Orders[0]
	expectedMessages := setup.TestData.ChatMessages[testOrder.ID]

	tests := []struct {
		name           string
		password       string
		orderID        int
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:           "get messages for existing order",
			password:       "test-worker-password",
			orderID:        testOrder.ID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var messages []api.ChatMessageResponse
				testutil.ParseJSONResponse(t, resp, &messages)
				if len(messages) < len(expectedMessages) {
					t.Errorf("Expected at least %d messages, got %d", len(expectedMessages), len(messages))
				}
				// Verify messages are ordered by created_at
				for i := 1; i < len(messages); i++ {
					if messages[i].CreatedAt < messages[i-1].CreatedAt {
						t.Error("Expected messages to be ordered by created_at ascending")
					}
				}
			},
		},
		{
			name:           "get messages for order with no messages",
			password:       "test-worker-password",
			orderID:        setup.TestData.Orders[3].ID, // Order 4 has no messages in test data
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var messages []api.ChatMessageResponse
				testutil.ParseJSONResponse(t, resp, &messages)
				if len(messages) != 0 {
					t.Errorf("Expected 0 messages, got %d", len(messages))
				}
			},
		},
		{
			name:           "get messages for non-existent order",
			password:       "test-worker-password",
			orderID:        99999,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get messages unauthenticated",
			password:       "",
			orderID:        testOrder.ID,
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/orders/" + strconv.Itoa(tt.orderID) + "/messages"
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "GET", path, tt.password, nil)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "GET", path, nil)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}
