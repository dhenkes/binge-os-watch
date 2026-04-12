package service

import (
	"context"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type CalendarServiceImpl struct {
	calendar model.CalendarRepository
}

var _ model.CalendarService = (*CalendarServiceImpl)(nil)

func NewCalendarService(calendar model.CalendarRepository) *CalendarServiceImpl {
	return &CalendarServiceImpl{calendar: calendar}
}

func (s *CalendarServiceImpl) Upcoming(ctx context.Context, userID string, filter model.CalendarFilter) ([]model.CalendarEntry, error) {
	return s.calendar.Upcoming(ctx, userID, filter)
}

func (s *CalendarServiceImpl) RecentlyReleased(ctx context.Context, userID string, filter model.CalendarFilter) ([]model.CalendarEntry, error) {
	return s.calendar.RecentlyReleased(ctx, userID, filter)
}
