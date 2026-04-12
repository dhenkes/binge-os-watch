package handler

import (
	"net/http"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type CalendarHandler struct {
	calendar model.CalendarService
}

func NewCalendarHandler(calendar model.CalendarService) *CalendarHandler {
	return &CalendarHandler{calendar: calendar}
}

func (h *CalendarHandler) Calendar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := model.CalendarFilter{
		MediaType: model.MediaType(q.Get("type")),
		Range:     q.Get("range"),
	}
	if filter.Range == "" {
		filter.Range = "7d"
	}
	upcoming, err := h.calendar.Upcoming(r.Context(), authUserID(r), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	recent, _ := h.calendar.RecentlyReleased(r.Context(), authUserID(r), filter)
	writeJSON(w, http.StatusOK, map[string]any{
		"upcoming": upcoming,
		"recent":   recent,
	})
}
