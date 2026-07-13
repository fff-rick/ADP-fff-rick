package template

import (
	"bytes"
	"fmt"
	"text/template"

	"adp/internal/domain/model"
)

// Engine manages command templates: registration, lookup, and rendering.
type Engine struct {
	templates map[string]model.CommandTemplate
}

// NewEngine creates an Engine preloaded with built-in templates.
func NewEngine() *Engine {
	e := &Engine{
		templates: make(map[string]model.CommandTemplate),
	}
	for _, t := range builtinTemplates() {
		e.templates[t.Code] = t
	}
	return e
}

// GetTemplate returns a template by code.
func (e *Engine) GetTemplate(code string) (model.CommandTemplate, bool) {
	t, ok := e.templates[code]
	return t, ok
}

// ListTemplates returns all registered templates.
func (e *Engine) ListTemplates() []model.CommandTemplate {
	result := make([]model.CommandTemplate, 0, len(e.templates))
	for _, t := range e.templates {
		result = append(result, t)
	}
	return result
}

// Render fills in template parameters and returns the concrete command.
func (e *Engine) Render(code string, params map[string]string) (model.CommandTemplate, string, error) {
	tmpl, ok := e.templates[code]
	if !ok {
		return model.CommandTemplate{}, "", fmt.Errorf("template not found: %s", code)
	}

	// Apply defaults and check required parameters.
	merged := make(map[string]string, len(tmpl.Parameters))
	for _, p := range tmpl.Parameters {
		val, provided := params[p.Name]
		if !provided || val == "" {
			if p.Default != "" {
				merged[p.Name] = p.Default
			} else if p.Required {
				return model.CommandTemplate{}, "", fmt.Errorf("required parameter missing: %s", p.Name)
			}
		} else {
			merged[p.Name] = val
		}
	}

	t, err := template.New("cmd").Parse(tmpl.Command)
	if err != nil {
		return model.CommandTemplate{}, "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, merged); err != nil {
		return model.CommandTemplate{}, "", fmt.Errorf("template render error: %w", err)
	}

	return tmpl, buf.String(), nil
}
