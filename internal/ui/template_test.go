package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateService(t *testing.T) {
	// Create a temporary directory for test templates
	tmpDir, err := os.MkdirTemp("", "templates")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test templates
	baseTemplate := `<!DOCTYPE html>
<html>
<head>
    {{block "head" .}}{{end}}
</head>
<body>
    {{block "content" .}}{{end}}
    {{block "scripts" .}}{{end}}
</body>
</html>`

	authTemplate := `{{define "head"}}<title>Login</title>{{end}}
{{define "content"}}<div>Login Form</div>{{end}}
{{define "scripts"}}<script>auth.js</script>{{end}}`

	ordersTemplate := `{{define "head"}}<title>Orders</title>{{end}}
{{define "content"}}<div>Orders List</div>{{end}}
{{define "scripts"}}<script>orders.js</script>{{end}}`

	// Write templates to files
	if err := os.WriteFile(filepath.Join(tmpDir, "base.gohtml"), []byte(baseTemplate), 0644); err != nil {
		t.Fatalf("Failed to write base template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "auth.gohtml"), []byte(authTemplate), 0644); err != nil {
		t.Fatalf("Failed to write auth template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "orders.gohtml"), []byte(ordersTemplate), 0644); err != nil {
		t.Fatalf("Failed to write orders template: %v", err)
	}

	// Create template service
	ts, err := NewTemplateService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create template service: %v", err)
	}

	// Test GetTemplateNames
	names := ts.GetTemplateNames()
	expectedNames := map[string]bool{
		"auth":   true,
		"orders": true,
	}
	if len(names) != len(expectedNames) {
		t.Errorf("Expected %d template names, got %d", len(expectedNames), len(names))
	}
	for _, name := range names {
		if !expectedNames[name] {
			t.Errorf("Unexpected template name: %s", name)
		}
	}

	// Test ExecuteTemplate for auth
	var authBuf bytes.Buffer
	err = ts.ExecuteTemplate(&authBuf, "auth", TemplateData{})
	if err != nil {
		t.Errorf("Failed to execute auth template: %v", err)
	}
	authOutput := authBuf.String()
	expectedAuthContent := []string{
		"<!DOCTYPE html>",
		"<title>Login</title>",
		"Login Form",
		"auth.js",
	}
	for _, content := range expectedAuthContent {
		if !strings.Contains(authOutput, content) {
			t.Errorf("Auth template output missing expected content: %s", content)
		}
	}

	// Test ExecuteTemplate for orders
	var ordersBuf bytes.Buffer
	err = ts.ExecuteTemplate(&ordersBuf, "orders", TemplateData{})
	if err != nil {
		t.Errorf("Failed to execute orders template: %v", err)
	}
	ordersOutput := ordersBuf.String()
	expectedOrdersContent := []string{
		"<!DOCTYPE html>",
		"<title>Orders</title>",
		"Orders List",
		"orders.js",
	}
	for _, content := range expectedOrdersContent {
		if !strings.Contains(ordersOutput, content) {
			t.Errorf("Orders template output missing expected content: %s", content)
		}
	}

	// Test non-existent template
	var buf bytes.Buffer
	err = ts.ExecuteTemplate(&buf, "nonexistent", TemplateData{})
	if err == nil {
		t.Error("Expected error for non-existent template, got nil")
	}
}
