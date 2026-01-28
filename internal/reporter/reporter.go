// Package reporter renders ScanReport into various output formats.
package reporter

import (
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/models"
)

// Render dispatches to the appropriate renderer based on format.
func Render(report *models.ScanReport, format string) (string, error) {
	switch format {
	case "json":
		return RenderJSON(report), nil
	case "markdown":
		return RenderMarkdown(report), nil
	case "html":
		return RenderHTML(report), nil
	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}
