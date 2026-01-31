package ui

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"time"
)

// TemplateData represents the data passed to templates
type TemplateData struct {
	IsAuthenticated bool
	UserRole        string
	Error           string
	MenuItems       []MenuItem
	Orders          []Order
	Order           Order // For order details page
	Messages        []Message
	Stats           Stats // Statistics for admin dashboard
	BuildVersion    string
}

// Stats represents statistics data
type Stats struct {
	TotalOrders int
	OrdersToday int
	Revenue     float64
}

// MenuItem represents a menu item in the system
type MenuItem struct {
	ID           string
	Name         string
	Price        float64
	DisplayOrder int
}

// Order represents an order in the system
type Order struct {
	ID           int
	CustomerName string
	Items        []OrderItem
	Status       string
	CreatedAt    time.Time
	Total        float64
}

// OrderItem represents an item in an order
type OrderItem struct {
	MenuItemID   string
	MenuItemName string
	Quantity     int
	Price        float64
}

// Message represents a chat message
type Message struct {
	ID        string
	Content   string
	CreatedAt string
}

// TemplateService handles template parsing and execution
type TemplateService struct {
	templates map[string]*template.Template
}

// NewTemplateService creates a new template service
func NewTemplateService(templatesDir string) (*TemplateService, error) {
	// Create a new template service
	ts := &TemplateService{
		templates: make(map[string]*template.Template),
	}

	// First, parse the base template
	baseTmpl, err := template.ParseFiles(filepath.Join(templatesDir, "base.gohtml"))
	if err != nil {
		return nil, fmt.Errorf("error parsing base template: %v", err)
	}

	// Get all template files except base.gohtml
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.gohtml"))
	if err != nil {
		return nil, fmt.Errorf("error finding template files: %v", err)
	}

	// Parse each template file
	for _, file := range files {
		if filepath.Base(file) == "base.gohtml" {
			continue
		}

		// Create a new template with the same name as the file (without extension)
		name := filepath.Base(file[:len(file)-len(filepath.Ext(file))])

		// Create a new template and add the base template first
		tmpl := template.New(name)
		_, err = tmpl.AddParseTree("base", baseTmpl.Tree)
		if err != nil {
			return nil, fmt.Errorf("error adding base template to %s: %v", name, err)
		}

		// Now parse the template file
		parsed, err := tmpl.ParseFiles(file)
		if err != nil {
			return nil, fmt.Errorf("error parsing template %s: %v", file, err)
		}

		// Verify that all required blocks are defined
		requiredBlocks := []string{"head", "content", "scripts"}
		for _, block := range requiredBlocks {
			if parsed.Lookup(block) == nil {
				return nil, fmt.Errorf("template %s is missing required block: %s", name, block)
			}
		}

		ts.templates[name] = parsed
	}

	return ts, nil
}

// ExecuteTemplate executes a template with the given data
func (ts *TemplateService) ExecuteTemplate(w io.Writer, name string, data TemplateData) error {
	tmpl, ok := ts.templates[name]
	if !ok {
		return fmt.Errorf("template %s not found", name)
	}
	// Execute using the base template name
	return tmpl.ExecuteTemplate(w, "base", data)
}

// GetTemplateNames returns a list of all available templates
func (ts *TemplateService) GetTemplateNames() []string {
	names := make([]string, 0, len(ts.templates))
	for name := range ts.templates {
		names = append(names, name)
	}
	return names
}
