package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// formatExt maps output format names to file extensions.
var formatExt = map[string]string{
	"json":     ".json",
	"markdown": ".md",
	"html":     ".html",
}

// MakeDefaultOutputPath generates a default output path: ./reports/<dbname>_<timestamp>.<ext>.
func MakeDefaultOutputPath(format, dbname string) string {
	ts := time.Now().Format("20060102_150405")
	ext := formatExt[format]
	name := dbname
	if name == "" {
		name = "mm-ready"
	}
	return filepath.Join("reports", name+"_"+ts+ext)
}

// MakeOutputPath inserts a timestamp into a user-provided output path.
// If the user provides "report.html", the result is "report_20260127_131504.html".
// If they provide a directory, the file is placed there with an auto-generated name.
func MakeOutputPath(userPath, format, dbname string) string {
	ts := time.Now().Format("20060102_150405")
	ext := formatExt[format]
	name := dbname
	if name == "" {
		name = "mm-ready"
	}

	// Check if userPath is a directory
	info, err := os.Stat(userPath)
	if err == nil && info.IsDir() {
		return filepath.Join(userPath, name+"_"+ts+ext)
	}

	base := userPath
	existingExt := filepath.Ext(userPath)
	if existingExt != "" {
		base = strings.TrimSuffix(userPath, existingExt)
	} else {
		existingExt = ext
	}
	return base + "_" + ts + existingExt
}
