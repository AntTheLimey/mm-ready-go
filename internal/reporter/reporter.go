// Package reporter renders ScanReport into various output formats.
package reporter

import (
	"fmt"

	"github.com/pgEdge/mm-ready-go/internal/models"
)

// ReportOptions controls report rendering behavior.
type ReportOptions struct {
	TodoList            bool
	TodoIncludeConsider bool
}

// DefaultReportOptions returns options with To Do list enabled.
func DefaultReportOptions() ReportOptions {
	return ReportOptions{TodoList: true}
}

// Render dispatches to the appropriate renderer based on format.
func Render(report *models.ScanReport, format string, opts ReportOptions) (string, error) {
	switch format {
	case "json":
		return RenderJSON(report), nil
	case "markdown":
		return RenderMarkdown(report), nil
	case "html":
		return RenderHTML(report, opts), nil
	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}
