package report

import (
	"bytes"
	"fmt"
	"html/template"
	"time"
)

type HTMLRenderer struct {
	tmpl *template.Template
}

func NewHTMLRenderer() *HTMLRenderer {
	tmpl := template.Must(template.New("report").Funcs(template.FuncMap{
		"scoreColor": func(score int) string {
			if score >= 80 {
				return "#10B981" // Emerald green
			} else if score >= 50 {
				return "#F59E0B" // Amber / orange
			}
			return "#EF4444" // Red
		},
		"severityBadgeClass": func(sev string) string {
			switch sev {
			case "critical":
				return "badge-critical"
			case "warning":
				return "badge-warning"
			default:
				return "badge-info"
			}
		},
		"effortBadgeClass": func(effort string) string {
			switch effort {
			case "low":
				return "badge-effort-low"
			case "medium":
				return "badge-effort-medium"
			default:
				return "badge-effort-high"
			}
		},
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return "N/A"
			}
			return t.Format("Jan 02, 2006 15:04 UTC")
		},
	}).Parse(htmlTemplateSource))

	return &HTMLRenderer{tmpl: tmpl}
}

func (r *HTMLRenderer) Render(data *ReportData) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render HTML report: %w", err)
	}
	return buf.Bytes(), nil
}

const htmlTemplateSource = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SEO Audit Report - {{.Domain}}</title>
  <style>
    :root {
      --bg: #0F172A;
      --card-bg: #1E293B;
      --border: #334155;
      --text: #F8FAFC;
      --text-muted: #94A3B8;
      --primary: #3B82F6;
      --success: #10B981;
      --warning: #F59E0B;
      --danger: #EF4444;
      --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background-color: var(--bg);
      color: var(--text);
      font-family: var(--font);
      line-height: 1.6;
      padding: 2rem 1rem;
    }
    .container { max-width: 1100px; margin: 0 auto; }
    .header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      border-bottom: 1px solid var(--border);
      padding-bottom: 1.5rem;
      margin-bottom: 2rem;
      flex-wrap: wrap;
      gap: 1rem;
    }
    .header h1 { font-size: 1.875rem; font-weight: 700; color: #fff; }
    .header .meta { color: var(--text-muted); font-size: 0.875rem; margin-top: 0.25rem; }
    .header .actions { display: flex; gap: 0.75rem; }
    .btn {
      display: inline-block;
      padding: 0.5rem 1rem;
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      color: var(--text);
      text-decoration: none;
      font-size: 0.875rem;
      font-weight: 500;
      transition: background 0.2s;
    }
    .btn:hover { background: #334155; }
    
    .alert-warning {
      background: rgba(245, 158, 11, 0.15);
      border: 1px solid var(--warning);
      color: #FDE68A;
      padding: 1rem;
      border-radius: 8px;
      margin-bottom: 2rem;
    }
    .alert-warning h3 { font-size: 1rem; margin-bottom: 0.5rem; color: #FBBF24; }
    .alert-warning ul { padding-left: 1.25rem; font-size: 0.875rem; }

    .score-grid {
      display: grid;
      grid-template-columns: 1fr 1fr 1fr 1fr;
      gap: 1.25rem;
      margin-bottom: 2.5rem;
    }
    @media (max-width: 860px) {
      .score-grid { grid-template-columns: 1fr 1fr; }
    }
    @media (max-width: 480px) {
      .score-grid { grid-template-columns: 1fr; }
    }
    .score-card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 1.5rem;
      text-align: center;
      position: relative;
    }
    .score-card.main-score {
      border-color: #3B82F6;
      background: linear-gradient(180deg, #1E293B 0%, #0F172A 100%);
    }
    .score-value {
      font-size: 3rem;
      font-weight: 800;
      line-height: 1;
      margin: 0.75rem 0;
    }
    .score-title { font-size: 0.875rem; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }

    .stats-bar {
      display: flex;
      justify-content: space-around;
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 1rem;
      margin-bottom: 2.5rem;
      flex-wrap: wrap;
      gap: 1rem;
    }
    .stat-item { text-align: center; }
    .stat-num { font-size: 1.25rem; font-weight: 700; }
    .stat-label { font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; }

    .section { margin-bottom: 3rem; }
    .section-title {
      font-size: 1.35rem;
      font-weight: 700;
      margin-bottom: 1.25rem;
      border-bottom: 1px solid var(--border);
      padding-bottom: 0.5rem;
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .section-desc { color: var(--text-muted); font-size: 0.875rem; margin-top: -0.75rem; margin-bottom: 1.25rem; }

    .issue-card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 1.25rem;
      margin-bottom: 1rem;
    }
    .issue-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 0.5rem;
      flex-wrap: wrap;
      gap: 0.5rem;
    }
    .issue-title { font-size: 1.1rem; font-weight: 600; }
    .issue-badges { display: flex; gap: 0.5rem; align-items: center; }
    .badge {
      font-size: 0.75rem;
      font-weight: 600;
      padding: 0.2rem 0.5rem;
      border-radius: 4px;
      text-transform: uppercase;
    }
    .badge-critical { background: rgba(239, 68, 68, 0.2); color: #FCA5A5; border: 1px solid #EF4444; }
    .badge-warning { background: rgba(245, 158, 11, 0.2); color: #FCD34D; border: 1px solid #F59E0B; }
    .badge-info { background: rgba(59, 130, 246, 0.2); color: #93C5FD; border: 1px solid #3B82F6; }
    .badge-priority { background: #4C1D95; color: #DDD6FE; border: 1px solid #8B5CF6; }
    .badge-effort-low { background: rgba(16, 185, 129, 0.2); color: #6EE7B7; }
    .badge-effort-medium { background: rgba(245, 158, 11, 0.2); color: #FCD34D; }
    .badge-effort-high { background: rgba(239, 68, 68, 0.2); color: #FCA5A5; }
    
    .issue-msg { color: var(--text-muted); font-size: 0.9rem; margin-bottom: 0.75rem; }
    details {
      background: #0F172A;
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 0.5rem 0.75rem;
      font-size: 0.85rem;
    }
    summary { cursor: pointer; color: var(--primary); font-weight: 500; }
    details ul { margin-top: 0.5rem; padding-left: 1.25rem; word-break: break-all; }
    details li { margin-bottom: 0.25rem; }

    table {
      width: 100%;
      border-collapse: collapse;
      background: var(--card-bg);
      border-radius: 8px;
      overflow: hidden;
      font-size: 0.875rem;
    }
    th, td {
      padding: 0.75rem 1rem;
      text-align: left;
      border-bottom: 1px solid var(--border);
    }
    th { background: #151E2E; color: var(--text-muted); font-weight: 600; text-transform: uppercase; font-size: 0.75rem; }
    tr:last-child td { border-bottom: none; }
    .url-cell { word-break: break-all; max-width: 400px; }
  </style>
</head>
<body>
  <div class="container">
    <!-- Header -->
    <div class="header">
      <div>
        <h1>SEO Audit Report: {{.Domain}}</h1>
        <div class="meta">
          Target URL: <strong>{{.NormalizedURL}}</strong> &bull;
          Audit Date: <strong>{{formatDate .GeneratedAt}}</strong> &bull;
          Version: <strong>v{{.Version}}</strong> &bull;
          Pages Analyzed: <strong>{{.TotalPagesScored}}</strong>
        </div>
      </div>
      <div class="actions">
        <a href="?format=json" class="btn" download="report.json">Download JSON</a>
        <a href="?format=pdf" class="btn" download="report.pdf">Download PDF</a>
      </div>
    </div>

    <!-- Warnings / Partial Completion Banner -->
    {{if .HasWarnings}}
    <div class="alert-warning">
      <h3>⚠️ Audit Completed With Warnings</h3>
      <p>Certain analysis components were skipped or timed out during this audit run:</p>
      <ul>
        {{range .Warnings}}
        <li>{{.}}</li>
        {{end}}
      </ul>
    </div>
    {{end}}

    <!-- Score Gauges -->
    <div class="score-grid">
      <div class="score-card main-score">
        <div class="score-title">Overall SEO Score</div>
        <div class="score-value" style="color: {{scoreColor .OverallScore}};">{{.OverallScore}}</div>
        <div class="stat-label">out of 100</div>
      </div>
      <div class="score-card">
        <div class="score-title">Technical Health</div>
        <div class="score-value" style="color: {{scoreColor .TechnicalScore}};">{{.TechnicalScore}}</div>
        <div class="stat-label">40% weight</div>
      </div>
      <div class="score-card">
        <div class="score-title">On-Page Quality</div>
        <div class="score-value" style="color: {{scoreColor .OnPageScore}};">{{.OnPageScore}}</div>
        <div class="stat-label">35% weight</div>
      </div>
      <div class="score-card">
        <div class="score-title">Content & Keywords</div>
        <div class="score-value" style="color: {{scoreColor .ContentScore}};">{{.ContentScore}}</div>
        <div class="stat-label">25% weight</div>
      </div>
    </div>

    <!-- Stats Bar -->
    <div class="stats-bar">
      <div class="stat-item">
        <div class="stat-num">{{.TotalFindings}}</div>
        <div class="stat-label">Total Findings</div>
      </div>
      <div class="stat-item">
        <div class="stat-num" style="color: var(--danger);">{{.CriticalCount}}</div>
        <div class="stat-label">Critical Issues</div>
      </div>
      <div class="stat-item">
        <div class="stat-num" style="color: var(--warning);">{{.WarningCount}}</div>
        <div class="stat-label">Warnings</div>
      </div>
      <div class="stat-item">
        <div class="stat-num" style="color: var(--primary);">{{.InfoCount}}</div>
        <div class="stat-label">Notices</div>
      </div>
      <div class="stat-item">
        <div class="stat-num">{{.TotalKeywords}}</div>
        <div class="stat-label">Target Keywords</div>
      </div>
    </div>

    <!-- Prioritized Issues Section -->
    <div class="section">
      <div class="section-title">
        <span>Prioritized Issues (Ranked by ROI)</span>
        <span style="font-size: 0.9rem; font-weight: normal; color: var(--text-muted);">{{len .PrioritizedIssues}} Actionable Issues</span>
      </div>
      <p class="section-desc">Ranked by Priority Score (Impact / Effort). High impact fixes with low implementation effort appear first.</p>

      {{range .PrioritizedIssues}}
      <div class="issue-card">
        <div class="issue-header">
          <div class="issue-title">{{.Title}}</div>
          <div class="issue-badges">
            <span class="badge badge-priority">Priority {{.PriorityScore}}</span>
            <span class="badge {{severityBadgeClass .Severity}}">{{.Severity}}</span>
            <span class="badge {{effortBadgeClass .EffortTier}}">{{.EffortTier}} Effort</span>
          </div>
        </div>
        <div class="issue-msg">{{.Message}}</div>
        <details>
          <summary>Affected Pages ({{.AffectedPagesCount}} {{if eq .AffectedPagesCount 1}}page{{else}}pages{{end}})</summary>
          <ul>
            {{range .AffectedURLs}}
            <li><a href="{{.}}" target="_blank" rel="noopener noreferrer" style="color: #93C5FD;">{{.}}</a></li>
            {{end}}
          </ul>
        </details>
      </div>
      {{else}}
      <div class="issue-card" style="text-align: center; color: var(--success); padding: 2rem;">
        🎉 No SEO issues found! Your site scored 100/100.
      </div>
      {{end}}
    </div>

    <!-- Keyword Profile Section -->
    {{if .KeywordMap}}
    <div class="section">
      <div class="section-title">Keyword Architecture & Topic Coverage</div>
      <p class="section-desc">Identified primary target topics across audited pages. Cannibalization instances: <strong>{{.CannibalizationCount}}</strong>.</p>
      <table>
        <thead>
          <tr>
            <th>Keyword Phrase</th>
            <th>Pages Count</th>
            <th>Mapped URLs</th>
          </tr>
        </thead>
        <tbody>
          {{range .KeywordMap}}
          <tr>
            <td><strong>{{.Keyword}}</strong></td>
            <td>{{.PageCount}}</td>
            <td class="url-cell">
              {{range .TargetPages}}
              <div>{{.}}</div>
              {{end}}
            </td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>
    {{end}}

    <!-- Per-Page Score Breakdown -->
    <div class="section">
      <div class="section-title">Per-Page Score Breakdown</div>
      <table>
        <thead>
          <tr>
            <th>Page URL</th>
            <th>Overall</th>
            <th>Technical</th>
            <th>On-Page</th>
            <th>Content</th>
            <th>Critical</th>
            <th>Warning</th>
          </tr>
        </thead>
        <tbody>
          {{range .PageBreakdown}}
          <tr>
            <td class="url-cell"><strong>{{.PageURL}}</strong></td>
            <td><strong style="color: {{scoreColor .OverallScore}};">{{.OverallScore}}</strong></td>
            <td>{{.TechnicalScore}}</td>
            <td>{{.OnPageScore}}</td>
            <td>{{.ContentScore}}</td>
            <td style="color: {{if gt .CriticalCount 0}}var(--danger){{end}};">{{.CriticalCount}}</td>
            <td style="color: {{if gt .WarningCount 0}}var(--warning){{end}};">{{.WarningCount}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>

  </div>
</body>
</html>
`
