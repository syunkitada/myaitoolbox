package entrypoint

import (
	"errors"
	"net/http"
	"strings"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

const (
	browserAuditDefaultLimit = 50
	browserAuditMaxLimit     = 100
	browserAuditMaxOffset    = 10000
)

func (s *Server) GetBrowserAudit(w http.ResponseWriter, r *http.Request, params api.GetBrowserAuditParams) {
	if err := authorizeBrowserRequest(r, browserActor(r), true); err != nil {
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	project := browserProject(r, "")
	if project == "" {
		writeErrorStatus(w, http.StatusBadRequest, errors.New("browser project is required"))
		return
	}
	query, err := browserAuditQuery(project, params)
	if err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}

	page := domain.BrowserAuditPage{
		Events: []domain.BrowserAuditEvent{},
		Limit:  query.Limit,
		Offset: query.Offset,
	}
	if s.browser.auditReader != nil {
		page, err = s.browser.auditReader.List(r.Context(), query)
		if err != nil {
			writeErrorStatus(w, http.StatusInternalServerError, errors.New("browser audit log is unavailable"))
			return
		}
	}

	events := make([]api.BrowserAuditEvent, 0, len(page.Events))
	for _, event := range page.Events {
		events = append(events, api.BrowserAuditEvent{
			Timestamp: event.Timestamp,
			Event:     event.Event,
			Project:   event.Project,
			Actor:     optionalString(event.Actor),
			Command:   optionalString(event.Command),
			Reason:    optionalString(event.Reason),
			Result:    event.Result,
			SessionId: optionalString(event.SessionID),
		})
	}
	writeJSONResponse(w, http.StatusOK, api.BrowserAuditResponse{
		Events:  events,
		HasMore: page.HasMore,
		Limit:   page.Limit,
		Offset:  page.Offset,
		Total:   page.Total,
	})
}

func browserAuditQuery(project string, params api.GetBrowserAuditParams) (domain.BrowserAuditQuery, error) {
	limit := browserAuditDefaultLimit
	if params.Limit != nil {
		limit = *params.Limit
		if limit < 1 || limit > browserAuditMaxLimit {
			return domain.BrowserAuditQuery{}, errors.New("browser audit limit must be between 1 and 100")
		}
	}
	offset := 0
	if params.Offset != nil {
		offset = *params.Offset
		if offset < 0 || offset > browserAuditMaxOffset {
			return domain.BrowserAuditQuery{}, errors.New("browser audit offset must be between 0 and 10000")
		}
	}
	query := domain.BrowserAuditQuery{
		Project:   project,
		SessionID: optionalStringValue(params.SessionId),
		Event:     optionalStringValue(params.Event),
		Actor:     optionalStringValue(params.Actor),
		Command:   optionalStringValue(params.Command),
		Result:    optionalStringValue(params.Result),
		Limit:     limit,
		Offset:    offset,
	}
	if params.From != nil {
		query.From = params.From.UTC()
	}
	if params.To != nil {
		query.To = params.To.UTC()
	}
	if !query.From.IsZero() && !query.To.IsZero() && query.From.After(query.To) {
		return domain.BrowserAuditQuery{}, errors.New("browser audit from must not be after to")
	}
	return query, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
