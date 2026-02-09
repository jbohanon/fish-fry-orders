package testing

import (
	"net/http"
	"testing"

	"git.nonahob.net/jacob/fish-fry-orders/internal/api"
	"git.nonahob.net/jacob/fish-fry-orders/testing/testutil"
)

func TestMenu_GetMenuItems(t *testing.T) {
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
			name:           "get menu items as worker",
			password:       "test-worker-password",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var items []api.MenuItemResponse
				testutil.ParseJSONResponse(t, resp, &items)
				// Should only return active items
				activeCount := 0
				for _, item := range setup.TestData.MenuItems {
					if item.IsActive {
						activeCount++
					}
				}
				if len(items) < activeCount {
					t.Errorf("Expected at least %d active items, got %d", activeCount, len(items))
				}
				// Verify all returned items are active
				for _, item := range items {
					if !item.IsActive {
						t.Errorf("Expected all items to be active, but found inactive item: %s", item.ID)
					}
				}
			},
		},
		{
			name:           "get menu items as admin",
			password:       "test-admin-password",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var items []api.MenuItemResponse
				testutil.ParseJSONResponse(t, resp, &items)
				if len(items) == 0 {
					t.Error("Expected at least one menu item")
				}
			},
		},
		{
			name:           "get menu items unauthenticated",
			password:       "",
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "GET", "/api/menu-items", tt.password, nil)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "GET", "/api/menu-items", nil)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestMenu_GetMenuItem(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	testItem := setup.TestData.MenuItems[0]

	tests := []struct {
		name           string
		password       string
		itemID         string
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:           "get existing menu item",
			password:       "test-worker-password",
			itemID:         testItem.ID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var item api.MenuItemResponse
				testutil.ParseJSONResponse(t, resp, &item)
				if item.ID != testItem.ID {
					t.Errorf("Expected item ID '%s', got '%s'", testItem.ID, item.ID)
				}
				if item.Name != testItem.Name {
					t.Errorf("Expected item name '%s', got '%s'", testItem.Name, item.Name)
				}
				if item.Price != testItem.Price {
					t.Errorf("Expected item price %.2f, got %.2f", testItem.Price, item.Price)
				}
			},
		},
		{
			name:           "get non-existent menu item",
			password:       "test-worker-password",
			itemID:         "non-existent-item",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get menu item unauthenticated",
			password:       "",
			itemID:         testItem.ID,
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/menu-items/" + tt.itemID
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

func TestMenu_CreateMenuItem(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	tests := []struct {
		name           string
		password       string
		request        api.CreateMenuItemRequest
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:     "create menu item with valid data",
			password: "test-admin-password",
			request: api.CreateMenuItemRequest{
				Name:     "New Test Item",
				Price:    9.99,
				IsActive: true,
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var item api.MenuItemResponse
				testutil.ParseJSONResponse(t, resp, &item)
				if item.ID == "" {
					t.Error("Expected item ID to be set")
				}
				if item.Name != "New Test Item" {
					t.Errorf("Expected item name 'New Test Item', got '%s'", item.Name)
				}
				if item.Price != 9.99 {
					t.Errorf("Expected item price 9.99, got %.2f", item.Price)
				}
				if !item.IsActive {
					t.Error("Expected item to be active")
				}
			},
		},
		{
			name:     "create menu item with empty name",
			password: "test-admin-password",
			request: api.CreateMenuItemRequest{
				Name:     "",
				Price:    9.99,
				IsActive: true,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "create menu item with zero price",
			password: "test-admin-password",
			request: api.CreateMenuItemRequest{
				Name:     "Test Item",
				Price:    0,
				IsActive: true,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "create menu item with negative price",
			password: "test-admin-password",
			request: api.CreateMenuItemRequest{
				Name:     "Test Item",
				Price:    -5.99,
				IsActive: true,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "create menu item unauthenticated",
			password: "",
			request: api.CreateMenuItemRequest{
				Name:     "Test Item",
				Price:    9.99,
				IsActive: true,
			},
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "POST", "/api/menu-items", tt.password, tt.request)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "POST", "/api/menu-items", tt.request)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestMenu_UpdateMenuItem(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	testItem := setup.TestData.MenuItems[0]

	tests := []struct {
		name           string
		password       string
		itemID         string
		request        api.UpdateMenuItemRequest
		expectedStatus int
		validateFunc   func(t *testing.T, resp *http.Response)
	}{
		{
			name:     "update menu item",
			password:  "test-admin-password",
			itemID:    testItem.ID,
			request: api.UpdateMenuItemRequest{
				Name:     "Updated Item Name",
				Price:    15.99,
				IsActive: func() *bool { b := true; return &b }(),
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var item api.MenuItemResponse
				testutil.ParseJSONResponse(t, resp, &item)
				if item.Name != "Updated Item Name" {
					t.Errorf("Expected item name 'Updated Item Name', got '%s'", item.Name)
				}
				if item.Price != 15.99 {
					t.Errorf("Expected item price 15.99, got %.2f", item.Price)
				}
			},
		},
		{
			name:     "update menu item to inactive",
			password: "test-admin-password",
			itemID:    testItem.ID,
			request: api.UpdateMenuItemRequest{
				Name:     testItem.Name,
				Price:    testItem.Price,
				IsActive: func() *bool { b := false; return &b }(),
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, resp *http.Response) {
				var item api.MenuItemResponse
				testutil.ParseJSONResponse(t, resp, &item)
				if item.IsActive {
					t.Error("Expected item to be inactive")
				}
			},
		},
		{
			name:     "update non-existent menu item",
			password: "test-admin-password",
			itemID:   "non-existent-item",
			request: api.UpdateMenuItemRequest{
				Name:  "Test",
				Price: 10.99,
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:     "update menu item unauthenticated",
			password: "",
			itemID:    testItem.ID,
			request: api.UpdateMenuItemRequest{
				Name:  "Test",
				Price: 10.99,
			},
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/menu-items/" + tt.itemID
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "PUT", path, tt.password, tt.request)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "PUT", path, tt.request)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, resp)
			}
		})
	}
}

func TestMenu_DeleteMenuItem(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	testItem := setup.TestData.MenuItems[0]

	tests := []struct {
		name           string
		password       string
		itemID         string
		expectedStatus int
	}{
		{
			name:           "delete existing menu item",
			password:       "test-admin-password",
			itemID:         testItem.ID,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "delete non-existent menu item",
			password:       "test-admin-password",
			itemID:         "non-existent-item",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "delete menu item unauthenticated",
			password:       "",
			itemID:         testItem.ID,
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/menu-items/" + tt.itemID
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "DELETE", path, tt.password, nil)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "DELETE", path, nil)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)
		})
	}
}

func TestMenu_UpdateMenuItemsOrder(t *testing.T) {
	setup := testutil.SetupTest(t)
	server := testutil.NewTestServer(setup.DBRepo)
	defer server.Close()

	tests := []struct {
		name           string
		password       string
		request        api.UpdateMenuItemsOrderRequest
		expectedStatus int
	}{
		{
			name:     "update menu items order",
			password:  "test-admin-password",
			request: api.UpdateMenuItemsOrderRequest{
				ItemOrders: map[string]int{
					setup.TestData.MenuItems[0].ID: 10,
					setup.TestData.MenuItems[1].ID: 20,
				},
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:     "update menu items order with empty map",
			password: "test-admin-password",
			request: api.UpdateMenuItemsOrderRequest{
				ItemOrders: map[string]int{},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "update menu items order unauthenticated",
			password: "",
			request: api.UpdateMenuItemsOrderRequest{
				ItemOrders: map[string]int{
					setup.TestData.MenuItems[0].ID: 10,
				},
			},
			expectedStatus: http.StatusFound, // Redirect to /auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.password != "" {
				resp = testutil.AuthenticatedRequest(t, server, "PUT", "/api/menu-items/order", tt.password, tt.request)
			} else {
				resp = testutil.UnauthenticatedRequest(t, server, "PUT", "/api/menu-items/order", tt.request)
			}

			testutil.AssertStatusCode(t, resp, tt.expectedStatus)
		})
	}
}
