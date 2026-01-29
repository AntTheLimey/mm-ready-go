package reporter

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/models"
)

const htmlCSS = `
/* ── Reset & base ─────────────────────────────────────────────── */
*, *::before, *::after { box-sizing: border-box; }
body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    margin: 0; padding: 0; color: #333; line-height: 1.6; background: #f9fafb;
}
/* ── Sidebar ──────────────────────────────────────────────────── */
.sidebar {
    position: fixed; top: 0; left: 0; width: 270px; height: 100vh;
    background: #1e293b; color: #e2e8f0; overflow-y: auto;
    padding: 20px 0; z-index: 100;
    display: flex; flex-direction: column;
}
.sidebar-header {
    padding: 0 20px 16px; border-bottom: 1px solid #334155;
    margin-bottom: 8px; font-size: 0.85em; color: #94a3b8;
}
.sidebar-header strong { color: #f1f5f9; font-size: 1.15em; }
.sidebar-nav { flex: 1; overflow-y: auto; }
.sidebar-footer {
    padding: 12px 20px; border-top: 1px solid #334155;
    font-size: 0.8em; color: #64748b;
}
.tree-section { margin-bottom: 2px; }
.tree-toggle {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 20px; cursor: pointer; user-select: none;
    font-weight: 600; font-size: 0.9em; transition: background 0.15s;
}
.tree-toggle:hover { background: #334155; }
.tree-toggle .arrow {
    display: inline-block; width: 12px; font-size: 0.7em;
    transition: transform 0.2s;
}
.tree-toggle .arrow.open { transform: rotate(90deg); }
.tree-badge {
    margin-left: auto; padding: 1px 7px; border-radius: 10px;
    font-size: 0.75em; font-weight: 700;
}
.tree-badge-critical { background: #dc2626; color: white; }
.tree-badge-warning { background: #d97706; color: white; }
.tree-badge-consider { background: #0891b2; color: white; }
.tree-badge-info { background: #2563eb; color: white; }
.tree-badge-errors { background: #991b1b; color: white; }
.tree-children { overflow: hidden; transition: max-height 0.25s ease; }
.tree-children.collapsed { max-height: 0 !important; }
.tree-child {
    display: block; padding: 5px 20px 5px 44px; color: #cbd5e1;
    text-decoration: none; font-size: 0.82em; transition: background 0.15s;
}
.tree-child:hover { background: #334155; color: #f1f5f9; }
.tree-child.active { background: #1d4ed8; color: white; }
.tree-child-count { color: #64748b; margin-left: 4px; }
.tree-link {
    display: block; padding: 8px 20px; color: #cbd5e1;
    text-decoration: none; font-weight: 600; font-size: 0.9em;
    transition: background 0.15s; margin-top: 4px;
    border-top: 1px solid #334155;
}
.tree-link:hover { background: #334155; color: #f1f5f9; }
.main {
    margin-left: 270px; padding: 32px 40px; max-width: 1000px;
}
h1 { border-bottom: 3px solid #2563eb; padding-bottom: 10px; margin-top: 0; }
h2 { color: #1e40af; margin-top: 2.5em; }
h3 { color: #374151; margin-top: 1.5em; }
h4 { color: #4b5563; margin-bottom: 4px; }
table { border-collapse: collapse; width: 100%; margin: 1em 0; }
th, td { border: 1px solid #d1d5db; padding: 8px 12px; text-align: left; }
th { background: #f3f4f6; }
code { background: #f3f4f6; padding: 2px 6px; border-radius: 3px; font-size: 0.9em; }
pre { background: #f3f4f6; padding: 12px; border-radius: 6px; overflow-x: auto;
      white-space: pre-wrap; word-wrap: break-word; }
blockquote { border-left: 4px solid #2563eb; margin: 1em 0; padding: 8px 16px;
             background: #eff6ff; }
hr { border: none; border-top: 1px solid #e5e7eb; margin: 1.5em 0; }
.badge { display: inline-block; padding: 2px 10px; border-radius: 4px;
         font-size: 0.8em; font-weight: bold; color: white; }
.badge-critical { background: #dc2626; }
.badge-warning { background: #d97706; }
.badge-consider { background: #0891b2; }
.badge-info { background: #2563eb; }
.summary-box { display: flex; gap: 16px; flex-wrap: wrap; margin: 1em 0; }
.summary-card { border: 1px solid #d1d5db; border-radius: 8px; padding: 16px 24px;
                text-align: center; min-width: 110px; background: white; }
.summary-card .number { font-size: 2em; font-weight: bold; }
.summary-card.critical .number { color: #dc2626; }
.summary-card.warning .number { color: #d97706; }
.summary-card.consider .number { color: #0891b2; }
.summary-card.info .number { color: #2563eb; }
.summary-card.passed .number { color: #16a34a; }
.finding-card {
    margin-bottom: 1.2em; padding: 14px 18px; border: 1px solid #e5e7eb;
    border-radius: 8px; background: white;
}
.finding-card p { margin: 6px 0; }
.finding-detail { white-space: pre-wrap; }
.todo-summary {
    padding: 12px 18px; border-radius: 8px; margin-bottom: 1.5em;
    font-weight: 600;
}
.todo-summary-red { background: #fef2f2; border: 1px solid #fecaca; color: #991b1b; }
.todo-summary-amber { background: #fffbeb; border: 1px solid #fde68a; color: #92400e; }
.todo-summary-cyan { background: #ecfeff; border: 1px solid #a5f3fc; color: #155e75; }
.todo-summary-green { background: #f0fdf4; border: 1px solid #bbf7d0; color: #166534; }
.todo-group-label {
    font-weight: 700; font-size: 0.9em; padding: 6px 12px; border-radius: 4px;
    margin: 1.2em 0 0.6em; display: inline-block;
}
.todo-group-critical { background: #fef2f2; color: #991b1b; }
.todo-group-warning { background: #fffbeb; color: #92400e; }
.todo-group-consider { background: #ecfeff; color: #155e75; }
.todo-item {
    display: flex; gap: 12px; padding: 10px 14px; border: 1px solid #e5e7eb;
    border-radius: 6px; margin-bottom: 8px; background: white;
    align-items: flex-start;
}
.todo-item.checked { opacity: 0.55; text-decoration: line-through; }
.todo-item input[type="checkbox"] {
    margin-top: 4px; width: 16px; height: 16px; flex-shrink: 0;
    accent-color: #2563eb; cursor: pointer;
}
.todo-content { flex: 1; }
.todo-title { font-weight: 600; font-size: 0.92em; }
.todo-object { font-size: 0.82em; color: #6b7280; }
.todo-remediation {
    font-size: 0.85em; color: #374151; margin-top: 4px;
    white-space: pre-wrap; word-wrap: break-word;
}
@media print {
    .sidebar { display: none; }
    .main { margin-left: 0; padding: 20px; max-width: 100%; }
    .finding-card, .todo-item { break-inside: avoid; }
    .todo-item.checked { opacity: 0.4; }
}
@media (max-width: 800px) {
    .sidebar { display: none; }
    .main { margin-left: 0; padding: 20px; }
}
`

const htmlJS = `
// Sidebar tree toggle
document.querySelectorAll('.tree-toggle').forEach(function(el) {
    el.addEventListener('click', function() {
        var children = this.nextElementSibling;
        var arrow = this.querySelector('.arrow');
        if (children && children.classList.contains('tree-children')) {
            children.classList.toggle('collapsed');
            arrow.classList.toggle('open');
        }
    });
});
// Smooth scroll for sidebar links
document.querySelectorAll('.tree-child, .tree-link').forEach(function(el) {
    el.addEventListener('click', function(e) {
        var href = this.getAttribute('href');
        if (href && href.startsWith('#')) {
            e.preventDefault();
            var target = document.getElementById(href.substring(1));
            if (target) {
                target.scrollIntoView({ behavior: 'smooth', block: 'start' });
            }
        }
    });
});
// Scroll tracking
(function() {
    var headings = document.querySelectorAll('h2[id], h3[id]');
    if (!headings.length) return;
    var sidebarLinks = {};
    document.querySelectorAll('.tree-child').forEach(function(link) {
        var href = link.getAttribute('href');
        if (href && href.startsWith('#')) {
            sidebarLinks[href.substring(1)] = link;
        }
    });
    var observer = new IntersectionObserver(function(entries) {
        entries.forEach(function(entry) {
            var link = sidebarLinks[entry.target.id];
            if (link) {
                if (entry.isIntersecting) {
                    document.querySelectorAll('.tree-child.active').forEach(function(c) {
                        c.classList.remove('active');
                    });
                    link.classList.add('active');
                    var section = link.closest('.tree-section');
                    if (section) {
                        var children = section.querySelector('.tree-children');
                        var arrow = section.querySelector('.arrow');
                        if (children && children.classList.contains('collapsed')) {
                            children.classList.remove('collapsed');
                            if (arrow) arrow.classList.add('open');
                        }
                    }
                }
            }
        });
    }, {
        rootMargin: '-10% 0px -80% 0px',
        threshold: 0
    });
    headings.forEach(function(heading) {
        if (sidebarLinks[heading.id]) {
            observer.observe(heading);
        }
    });
})();
// To Do checkboxes
document.querySelectorAll('.todo-item input[type="checkbox"]').forEach(function(cb) {
    cb.addEventListener('change', function() {
        this.closest('.todo-item').classList.toggle('checked', this.checked);
        updateTodoCount();
    });
});
function updateTodoCount() {
    var total = document.querySelectorAll('.todo-item').length;
    var done = document.querySelectorAll('.todo-item.checked').length;
    var counter = document.getElementById('todo-counter');
    if (counter) {
        counter.textContent = done + ' of ' + total + ' completed';
    }
}
`

// esc HTML-escapes a string.
func esc(text string) string {
	return html.EscapeString(text)
}

// slug converts a label to a URL-safe anchor fragment.
func slug(text string) string {
	s := strings.ToLower(text)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

// pluralS returns "s" if n != 1, otherwise "".
func pluralS(n int) string {
	if n != 1 {
		return "s"
	}
	return ""
}

// sevBadgeClass returns (badge CSS class, tree-badge CSS class) for a severity.
func sevBadgeClass(sev models.Severity) (string, string) {
	switch sev {
	case models.SeverityCritical:
		return "badge-critical", "tree-badge-critical"
	case models.SeverityWarning:
		return "badge-warning", "tree-badge-warning"
	case models.SeverityConsider:
		return "badge-consider", "tree-badge-consider"
	case models.SeverityInfo:
		return "badge-info", "tree-badge-info"
	default:
		return "badge-info", "tree-badge-info"
	}
}

// sevCatEntry holds findings grouped by category within a severity level.
type sevCatEntry struct {
	severity models.Severity
	// categories is ordered alphabetically; each entry is (category name, findings).
	categories []catFindings
}

type catFindings struct {
	category string
	findings []models.Finding
}

// buildSevCatMap groups all findings by severity then category (sorted).
func buildSevCatMap(allFindings []models.Finding) []sevCatEntry {
	sevOrder := []models.Severity{
		models.SeverityCritical,
		models.SeverityWarning,
		models.SeverityConsider,
		models.SeverityInfo,
	}

	var result []sevCatEntry
	for _, sev := range sevOrder {
		catMap := make(map[string][]models.Finding)
		for _, f := range allFindings {
			if f.Severity == sev {
				catMap[f.Category] = append(catMap[f.Category], f)
			}
		}
		if len(catMap) == 0 {
			continue
		}
		// Sort category keys.
		cats := make([]string, 0, len(catMap))
		for c := range catMap {
			cats = append(cats, c)
		}
		sort.Strings(cats)

		entry := sevCatEntry{severity: sev}
		for _, c := range cats {
			entry.categories = append(entry.categories, catFindings{category: c, findings: catMap[c]})
		}
		result = append(result, entry)
	}
	return result
}

// RenderHTML renders the report as a standalone HTML page with sidebar navigation and To Do list.
func RenderHTML(report *models.ScanReport) string {
	allFindings := report.Findings()
	sevCatMap := buildSevCatMap(allFindings)

	// Collect errors.
	var errors []models.CheckResult
	for _, r := range report.Results {
		if r.Error != "" {
			errors = append(errors, r)
		}
	}

	// Collect to-do items (CRITICAL, WARNING, CONSIDER findings with remediation).
	var todoItems []models.Finding
	for _, sev := range []models.Severity{models.SeverityCritical, models.SeverityWarning, models.SeverityConsider} {
		for _, f := range allFindings {
			if f.Severity == sev && f.Remediation != "" {
				todoItems = append(todoItems, f)
			}
		}
	}

	collapsedSeverities := map[models.Severity]bool{
		models.SeverityConsider: true,
		models.SeverityInfo:     true,
	}

	// ── Build sidebar ────────────────────────────────────────────
	var sb []string
	sb = append(sb, `<div class="sidebar">`)
	sb = append(sb, `<div class="sidebar-header">`)
	sb = append(sb, `<strong>MM-Ready Report</strong><br>`)
	sb = append(sb, esc(report.Database))
	sb = append(sb, `</div>`)
	sb = append(sb, `<nav class="sidebar-nav">`)

	for _, entry := range sevCatMap {
		sevLabel := entry.severity.String()
		sevSlug := slug(sevLabel)
		sevCount := 0
		for _, cf := range entry.categories {
			sevCount += len(cf.findings)
		}
		_, treeBadge := sevBadgeClass(entry.severity)

		collapsed := ""
		arrowCls := "arrow open"
		if collapsedSeverities[entry.severity] {
			collapsed = "collapsed"
			arrowCls = "arrow"
		}

		sb = append(sb, `<div class="tree-section">`)
		sb = append(sb, `<div class="tree-toggle">`)
		sb = append(sb, fmt.Sprintf(`<span class="%s">&#9654;</span>`, arrowCls))
		sb = append(sb, sevLabel)
		sb = append(sb, fmt.Sprintf(`<span class="tree-badge %s">%d</span>`, treeBadge, sevCount))
		sb = append(sb, `</div>`)
		sb = append(sb, fmt.Sprintf(`<div class="tree-children %s" style="max-height:500px">`, collapsed))
		for _, cf := range entry.categories {
			anchor := fmt.Sprintf("sev-%s-%s", sevSlug, slug(cf.category))
			sb = append(sb, fmt.Sprintf(
				`<a class="tree-child" href="#%s">%s<span class="tree-child-count">(%d)</span></a>`,
				anchor, esc(cf.category), len(cf.findings),
			))
		}
		sb = append(sb, `</div>`)
		sb = append(sb, `</div>`)
	}

	if len(errors) > 0 {
		sb = append(sb, fmt.Sprintf(
			`<a class="tree-link" href="#errors">Errors <span class="tree-badge tree-badge-errors">%d</span></a>`,
			len(errors),
		))
	}
	if len(todoItems) > 0 {
		sb = append(sb, fmt.Sprintf(
			`<a class="tree-link" href="#todo">To Do List <span class="tree-badge tree-badge-warning">%d</span></a>`,
			len(todoItems),
		))
	}

	sb = append(sb, `</nav>`)
	sb = append(sb, `<div class="sidebar-footer">mm-ready v0.1.0</div>`)
	sb = append(sb, `</div>`)

	// ── Build main content ───────────────────────────────────────
	var main []string
	main = append(main, `<div class="main">`)
	main = append(main, `<h1>MM-Ready: Spock 5 Readiness Report</h1>`)
	main = append(main, fmt.Sprintf(`<p><strong>Database:</strong> %s<br>`, esc(report.Database)))
	if report.ScanMode == "analyze" {
		main = append(main, fmt.Sprintf(`<strong>Source File:</strong> %s<br>`, esc(report.Host)))
	} else {
		main = append(main, fmt.Sprintf(`<strong>Host:</strong> %s:%d<br>`, esc(report.Host), report.Port))
	}
	main = append(main, fmt.Sprintf(`<strong>PostgreSQL:</strong> %s<br>`, esc(report.PGVersion)))
	main = append(main, fmt.Sprintf(`<strong>Scan Time:</strong> %s<br>`, report.Timestamp.Format("2006-01-02 15:04:05 UTC")))
	main = append(main, fmt.Sprintf(`<strong>Mode:</strong> %s<br>`, esc(report.ScanMode)))
	main = append(main, fmt.Sprintf(`<strong>Target:</strong> Spock %s</p>`, report.SpockTarget))

	main = append(main, `<div class="summary-box">`)
	main = append(main, fmt.Sprintf(`<div class="summary-card"><div class="number">%d</div>Checks Run</div>`, report.ChecksTotal()))
	main = append(main, fmt.Sprintf(`<div class="summary-card passed"><div class="number">%d</div>Passed</div>`, report.ChecksPassed()))
	main = append(main, fmt.Sprintf(`<div class="summary-card critical"><div class="number">%d</div>Critical</div>`, report.CriticalCount()))
	main = append(main, fmt.Sprintf(`<div class="summary-card warning"><div class="number">%d</div>Warnings</div>`, report.WarningCount()))
	main = append(main, fmt.Sprintf(`<div class="summary-card consider"><div class="number">%d</div>Consider</div>`, report.ConsiderCount()))
	main = append(main, fmt.Sprintf(`<div class="summary-card info"><div class="number">%d</div>Info</div>`, report.InfoCount()))
	main = append(main, `</div>`)

	critCount := report.CriticalCount()
	warnCount := report.WarningCount()
	if critCount == 0 && warnCount == 0 {
		main = append(main, `<blockquote style="border-left-color: #16a34a; background: #f0fdf4;">`)
		main = append(main, `<strong>READY</strong> — No critical or warning issues found.`)
	} else if critCount == 0 {
		main = append(main, `<blockquote style="border-left-color: #d97706; background: #fffbeb;">`)
		main = append(main, `<strong>CONDITIONALLY READY</strong> — No critical issues, but warnings should be reviewed.`)
	} else {
		main = append(main, `<blockquote style="border-left-color: #dc2626; background: #fef2f2;">`)
		main = append(main, fmt.Sprintf(`<strong>NOT READY</strong> — %d critical issue(s) must be resolved.`, critCount))
	}
	main = append(main, `</blockquote>`)

	// Findings by severity / category.
	for _, entry := range sevCatMap {
		sevLabel := entry.severity.String()
		sevSlug := slug(sevLabel)
		sevCount := 0
		for _, cf := range entry.categories {
			sevCount += len(cf.findings)
		}
		badgeCls, _ := sevBadgeClass(entry.severity)

		main = append(main, fmt.Sprintf(`<h2 id="sev-%s"><span class="badge %s">%s</span> (%d)</h2>`, sevSlug, badgeCls, sevLabel, sevCount))

		for _, cf := range entry.categories {
			anchor := fmt.Sprintf("sev-%s-%s", sevSlug, slug(cf.category))
			main = append(main, fmt.Sprintf(`<h3 id="%s">%s (%d)</h3>`, anchor, esc(cf.category), len(cf.findings)))

			for _, finding := range cf.findings {
				main = append(main, `<div class="finding-card">`)
				main = append(main, fmt.Sprintf(`<h4>%s</h4>`, esc(finding.Title)))
				if finding.ObjectName != "" {
					main = append(main, fmt.Sprintf(`<p><strong>Object:</strong> <code>%s</code></p>`, esc(finding.ObjectName)))
				}
				main = append(main, fmt.Sprintf(`<p class="finding-detail">%s</p>`, esc(finding.Detail)))
				if finding.Remediation != "" {
					main = append(main, fmt.Sprintf(`<p><strong>Remediation:</strong></p><pre>%s</pre>`, esc(finding.Remediation)))
				}
				main = append(main, `</div>`)
			}
		}
	}

	// Errors section.
	if len(errors) > 0 {
		main = append(main, `<h2 id="errors">Errors</h2>`)
		main = append(main, `<ul>`)
		for _, r := range errors {
			main = append(main, fmt.Sprintf(`<li><strong>%s/%s</strong>: %s</li>`, esc(r.Category), esc(r.CheckName), esc(r.Error)))
		}
		main = append(main, `</ul>`)
	}

	// To Do list section.
	if len(todoItems) > 0 {
		var critTodos, warnTodos, considerTodos []models.Finding
		for _, f := range todoItems {
			switch f.Severity {
			case models.SeverityCritical:
				critTodos = append(critTodos, f)
			case models.SeverityWarning:
				warnTodos = append(warnTodos, f)
			case models.SeverityConsider:
				considerTodos = append(considerTodos, f)
			}
		}

		main = append(main, `<h2 id="todo">To Do List</h2>`)

		var cssClass string
		if len(critTodos) > 0 {
			cssClass = "todo-summary todo-summary-red"
		} else if len(warnTodos) > 0 {
			cssClass = "todo-summary todo-summary-amber"
		} else if len(considerTodos) > 0 {
			cssClass = "todo-summary todo-summary-cyan"
		} else {
			cssClass = "todo-summary todo-summary-green"
		}

		var parts []string
		if len(critTodos) > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", len(critTodos)))
		}
		if len(warnTodos) > 0 {
			parts = append(parts, fmt.Sprintf("%d warning%s", len(warnTodos), pluralS(len(warnTodos))))
		}
		if len(considerTodos) > 0 {
			parts = append(parts, fmt.Sprintf("%d to consider", len(considerTodos)))
		}

		main = append(main, fmt.Sprintf(`<div class="%s">`, cssClass))
		main = append(main, fmt.Sprintf(`%d item%s to address (%s)`, len(todoItems), pluralS(len(todoItems)), strings.Join(parts, ", ")))
		main = append(main, fmt.Sprintf(` &mdash; <span id="todo-counter">0 of %d completed</span>`, len(todoItems)))
		main = append(main, `</div>`)

		type todoGroup struct {
			severity models.Severity
			items    []models.Finding
			label    string
			cls      string
		}
		groups := []todoGroup{
			{models.SeverityCritical, critTodos, "CRITICAL", "todo-group-critical"},
			{models.SeverityWarning, warnTodos, "WARNING", "todo-group-warning"},
			{models.SeverityConsider, considerTodos, "CONSIDER", "todo-group-consider"},
		}

		for _, g := range groups {
			if len(g.items) == 0 {
				continue
			}
			main = append(main, fmt.Sprintf(`<div class="todo-group-label %s">%s</div>`, g.cls, g.label))
			for _, finding := range g.items {
				objHTML := ""
				if finding.ObjectName != "" {
					objHTML = fmt.Sprintf(`<div class="todo-object"><code>%s</code></div>`, esc(finding.ObjectName))
				}
				main = append(main, fmt.Sprintf(
					`<div class="todo-item">`+
						`<input type="checkbox">`+
						`<div class="todo-content">`+
						`<div class="todo-title">%s</div>`+
						`%s`+
						`<div class="todo-remediation">%s</div>`+
						`</div></div>`,
					esc(finding.Title), objHTML, esc(finding.Remediation),
				))
			}
		}
	}

	main = append(main, `<hr>`)
	main = append(main, `<p><em>Generated by mm-ready v0.1.0</em></p>`)
	main = append(main, `</div>`)

	// ── Assemble document ────────────────────────────────────────
	var doc []string
	doc = append(doc, `<!DOCTYPE html>`)
	doc = append(doc, `<html lang="en">`)
	doc = append(doc, `<head>`)
	doc = append(doc, `<meta charset="UTF-8">`)
	doc = append(doc, `<meta name="viewport" content="width=device-width, initial-scale=1.0">`)
	doc = append(doc, fmt.Sprintf(`<title>MM-Ready Report: %s</title>`, esc(report.Database)))
	doc = append(doc, fmt.Sprintf(`<style>%s</style>`, htmlCSS))
	doc = append(doc, `</head>`)
	doc = append(doc, `<body>`)
	doc = append(doc, sb...)
	doc = append(doc, main...)
	doc = append(doc, fmt.Sprintf(`<script>%s</script>`, htmlJS))
	doc = append(doc, `</body></html>`)

	return strings.Join(doc, "\n")
}
