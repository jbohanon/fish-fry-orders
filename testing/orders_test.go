package testing

import (
	"net/http"
	"strconv"
	"testing"

	"git.nonahob.net/jacob/fish-fry-orders/internal/api"
	"git.nonahob.net/jacob/fish-fry-orders/testing/testutil"
)

func TestOrders_CreateOrder(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	tests := []struct {
		name           string
		password       string
		request        api.CreateOrderRequest
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:     "create order with valid data",
			password: "test-worker-password",
			request: api.CreateOrderRequest{
				VehicleDescription: "Green Subaru Outback",
				Items: []api.CreateOrderItemRequest{
					{MenuItemID: "test-baked-fish", Quantity: 2},
					{MenuItemID: "test-kids-pizza", Quantity: 1},
				},
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var order api.OrderResponse
				testutil.ParseJSONResponse(t, resp, &order)
				if order.ID == 0 {
					t.Error("Expected order ID to be set")
				}
				if order.VehicleDescription != "Green Subaru Outback" {
					t.Errorf("Expected vehicle description 'Green Subaru Outback', got '%s'", order.VehicleDescription)
				}
				if order.Status != "new" {
					t.Errorf("Expected status 'new', got '%s'", order.Status)
				}
				if len(order.Items) != 2 {
					t.Errorf("Expected 2 items, got %d", len(order.Items))
				}
			},
		},
		{
			name:     "create order with customerName (camelCase)",
			password: "test-worker-password",
			request: api.CreateOrderRequest{
				CustomerName: "Blue Honda Civic",
				Items: []api.CreateOrderItemRequest{
					{MenuItemId: "test-fried-fish", Quantity: 1},
				},
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var order api.OrderResponse
				testutil.ParseJSONResponse(t, resp, &order)
				if order.CustomerName != "Blue Honda Civic" {
					t.Errorf("Expected customer name 'Blue Honda Civic', got '%s'", order.CustomerName)
				}
			},
		},
		{
			name:     "create order with empty vehicle description",
			password: "test-worker-password",
			request: api.CreateOrderRequest{
				Items: []api.CreateOrderItemRequest{
					{MenuItemID: "test-baked-fish", Quantity: 1},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "create order with no items",
			password: "test-worker-password",
			request: api.CreateOrderRequest{
				VehicleDescription: "Test Vehicle",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "create order unauthenticated",
			password: "",
			request: api.CreateOrderRequest{
				VehicleDescription: "Test Vehicle",
				Items: []api.CreateOrderItemRequest{
					{MenuItemID: "test-baked-fish", Quantity: 1},
				},
			},
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "POST", "/api/orders", tt.password, tt.request)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "POST", "/api/orders", tt.request)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestOrders_GetOrders(t *testing.T) {
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
			name:           "get all orders as worker",
			password:       "test-worker-password",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var orders []api.OrderResponse
				testutil.ParseJSONResponse(t, resp, &orders)
				if len(orders) < len(setup.TestData.Orders) {
					t.Errorf("Expected at least %d orders, got %d", len(setup.TestData.Orders), len(orders))
				}
				// Verify orders are sorted correctly (IN_PROGRESS first, then NEW, then COMPLETED)
				foundInProgress := false
				foundNew := false
				for i, order := range orders {
					if order.Status == "in-progress" {
						foundInProgress = true
					}
					if order.Status == "new" && foundInProgress {
						t.Error("NEW orders should come after IN_PROGRESS orders")
					}
					if order.Status == "completed" {
						if !foundNew && !foundInProgress {
							t.Error("COMPLETED orders should come last")
						}
					}
					// Check that each order has items
					if len(order.Items) == 0 {
						t.Errorf("Order %d has no items", order.ID)
					}
					// Check ordering by ID within same status
					if i > 0 && orders[i-1].Status == order.Status && orders[i-1].ID > order.ID {
						t.Errorf("Orders with same status should be ordered by ID ascending")
					}
				}
			},
		},
		{
			name:           "get all orders as admin",
			password:       "test-admin-password",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var orders []api.OrderResponse
				testutil.ParseJSONResponse(t, resp, &orders)
				if len(orders) < len(setup.TestData.Orders) {
					t.Errorf("Expected at least %d orders, got %d", len(setup.TestData.Orders), len(orders))
				}
			},
		},
		{
			name:           "get orders unauthenticated",
			password:       "",
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "GET", "/api/orders", tt.password, nil)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "GET", "/api/orders", nil)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestOrders_GetOrder(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	testOrder := setup.TestData.Orders[0]

	tests := []struct {
		name           string
		password       string
		orderID        int
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:           "get existing order",
			password:       "test-worker-password",
			orderID:        testOrder.ID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var order api.OrderResponse
				testutil.ParseJSONResponse(t, resp, &order)
				if order.ID != testOrder.ID {
					t.Errorf("Expected order ID %d, got %d", testOrder.ID, order.ID)
				}
				if order.VehicleDescription != testOrder.VehicleDescription {
					t.Errorf("Expected vehicle description '%s', got '%s'", testOrder.VehicleDescription, order.VehicleDescription)
				}
				expectedItems := setup.TestData.OrderItems[testOrder.ID]
				if len(order.Items) != len(expectedItems) {
					t.Errorf("Expected %d items, got %d", len(expectedItems), len(order.Items))
				}
			},
		},
		{
			name:           "get non-existent order",
			password:       "test-worker-password",
			orderID:        99999,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get order unauthenticated",
			password:       "",
			orderID:        testOrder.ID,
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/orders/" + strconv.Itoa(tt.orderID)
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

func TestOrders_UpdateOrderStatus(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	testOrder := setup.TestData.Orders[0]

	tests := []struct {
		name           string
		password       string
		orderID        int
		status         string
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:           "update order status to in-progress",
			password:       "test-worker-password",
			orderID:        testOrder.ID,
			status:         "in-progress",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var order api.OrderResponse
				testutil.ParseJSONResponse(t, resp, &order)
				if order.Status != "in-progress" {
					t.Errorf("Expected status 'in-progress', got '%s'", order.Status)
				}
			},
		},
		{
			name:           "update order status to completed",
			password:       "test-worker-password",
			orderID:        testOrder.ID,
			status:         "completed",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var order api.OrderResponse
				testutil.ParseJSONResponse(t, resp, &order)
				if order.Status != "completed" {
					t.Errorf("Expected status 'completed', got '%s'", order.Status)
				}
			},
		},
		{
			name:           "update order status with invalid status",
			password:       "test-worker-password",
			orderID:        testOrder.ID,
			status:         "invalid-status",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "update non-existent order",
			password:       "test-worker-password",
			orderID:        99999,
			status:         "in-progress",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "update order unauthenticated",
			password:       "",
			orderID:        testOrder.ID,
			status:         "in-progress",
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/orders/" + strconv.Itoa(tt.orderID) + "/status"
			body := map[string]string{"status": tt.status}
			
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "PUT", path, tt.password, body)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "PUT", path, body)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestOrders_PurgeOrders(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	tests := []struct {
		name           string
		password       string
		scope          string
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:           "purge today's orders",
			password:       "test-admin-password",
			scope:          "today",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var result map[string]interface{}
				testutil.ParseJSONResponse(t, resp, &result)
				if deleted, ok := result["deleted"].(float64); ok {
					if deleted < 0 {
						t.Error("Expected deleted count to be non-negative")
					}
				} else {
					t.Error("Expected 'deleted' field in response")
				}
			},
		},
		{
			name:           "purge all orders",
			password:       "test-admin-password",
			scope:          "all",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var result map[string]interface{}
				testutil.ParseJSONResponse(t, resp, &result)
				if deleted, ok := result["deleted"].(float64); ok {
					if deleted < 0 {
						t.Error("Expected deleted count to be non-negative")
					}
				} else {
					t.Error("Expected 'deleted' field in response")
				}
			},
		},
		{
			name:           "purge with invalid scope",
			password:       "test-admin-password",
			scope:          "invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "purge orders unauthenticated",
			password:       "",
			scope:          "all",
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]string{"scope": tt.scope}
			
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "DELETE", "/api/orders/purge", tt.password, body)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "DELETE", "/api/orders/purge", body)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}
