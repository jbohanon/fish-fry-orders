package templates

import (
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

type Template struct {
	templates *template.Template
}

func NewTemplate() (*Template, error) {
	t := &Template{
		templates: template.New(""),
	}

	// Add template functions if needed
	t.templates.Funcs(template.FuncMap{
		// Add any custom template functions here
	})

	// Parse all template files
	patterns := []string{
		"internal/ui/templates/views/*.gohtml",
	}

	for _, pattern := range patterns {
		if _, err := t.templates.ParseGlob(pattern); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// Render implements echo.Renderer
func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}
