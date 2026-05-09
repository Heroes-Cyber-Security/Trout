package ui

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

var parsedTemplates *template.Template

func init() {
	parsedTemplates = template.Must(template.ParseFS(templateFS, "templates/*.html"))
}
