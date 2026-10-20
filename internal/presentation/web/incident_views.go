package web

import (
	"html/template"
	"io"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

var actionPanel = template.Must(template.New("actions").Parse(`
<section class="incident-actions" aria-labelledby="actions-heading">
  <header>
    <h2 id="actions-heading">Actions</h2>
    <p>Owned work confirmed for this incident.</p>
  </header>
  {{if .}}
  <div class="action-list">
    {{range .}}
    <article class="action-card" data-status="{{.Action.Status}}">
      <div class="action-card__title">
        <h3>{{.Action.Title}}</h3>
        {{if .Overdue}}<strong class="action-card__overdue">Overdue</strong>{{end}}
      </div>
      <dl>
        <div><dt>Owner</dt><dd>{{.Action.OwnerHumanID}}</dd></div>
        <div><dt>Status</dt><dd>{{.Action.Status}}</dd></div>
        <div><dt>Priority</dt><dd>{{.Action.Priority}}</dd></div>
        {{if .Action.DueAt}}<div><dt>Due</dt><dd>{{.Action.DueAt}}</dd></div>{{end}}
        <div><dt>Source</dt><dd>{{.Action.SourceRef}}</dd></div>
        {{if .Action.ChecklistTemplateID}}<div><dt>Checklist</dt><dd>{{.Action.ChecklistTemplateID}} / {{.Action.ChecklistStepID}}</dd></div>{{end}}
      </dl>
      {{if .Action.Notes}}<p class="action-card__notes">{{.Action.Notes}}</p>{{end}}
      {{if .Action.SkipReason}}<p class="action-card__skip"><strong>Skipped:</strong> {{.Action.SkipReason}}</p>{{end}}
      {{if .Action.EvidenceRefs}}<div class="action-card__evidence"><strong>Evidence</strong><ul>{{range .Action.EvidenceRefs}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
    </article>
    {{end}}
  </div>
  {{else}}
  <p>No confirmed actions.</p>
  {{end}}
</section>`))

var timelinePanel = template.Must(template.New("timeline").Parse(`
<section class="incident-timeline" aria-labelledby="timeline-heading">
  <header><h2 id="timeline-heading">Timeline</h2></header>
  {{if .}}
  <ol class="timeline-list">
    {{range .}}
    <li class="timeline-entry">
      <strong>{{.Summary}}</strong>
      <dl>
        {{if .OccurredAt}}<div><dt>Occurred</dt><dd>{{.OccurredAt}}</dd></div>{{end}}
        {{if .FlaggedAt}}<div><dt>Flagged</dt><dd>{{.FlaggedAt}}</dd></div>{{end}}
        <div><dt>Recorded</dt><dd>{{.RecordedAt}}</dd></div>
        {{range .Awareness}}<div><dt>Aware — {{.Subject}}</dt><dd>{{.At}}</dd></div>{{end}}
        <div><dt>Record</dt><dd>{{.RecordType}} / {{.Governance}}</dd></div>
      </dl>
    </li>
    {{end}}
  </ol>
  {{else}}
  <p>No timeline entries.</p>
  {{end}}
</section>`))

func RenderActions(w io.Writer, actions []domain.ActionView) error {
	return actionPanel.Execute(w, actions)
}

func RenderTimeline(w io.Writer, entries []domain.TimelineEntry) error {
	return timelinePanel.Execute(w, entries)
}
