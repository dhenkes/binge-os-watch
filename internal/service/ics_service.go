package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// ICSServiceImpl generates iCalendar feeds from calendar entries.
type ICSServiceImpl struct {
	calendar model.CalendarService
}

func NewICSService(calendar model.CalendarService) *ICSServiceImpl {
	return &ICSServiceImpl{calendar: calendar}
}

// GenerateFeed builds an iCalendar (RFC 5545) feed for the given user.
func (s *ICSServiceImpl) GenerateFeed(ctx context.Context, userID string) (string, error) {
	filter := model.CalendarFilter{Range: "90d"}
	upcoming, err := s.calendar.Upcoming(ctx, userID, filter)
	if err != nil {
		return "", fmt.Errorf("fetching upcoming: %w", err)
	}
	recent, err := s.calendar.RecentlyReleased(ctx, userID, filter)
	if err != nil {
		return "", fmt.Errorf("fetching recent: %w", err)
	}

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//binge-os-watch//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("X-WR-CALNAME:binge-os-watch\r\n")

	now := time.Now().UTC().Format("20060102T150405Z")

	entries := append(upcoming, recent...)
	for _, e := range entries {
		if e.ReleaseDate == "" {
			continue
		}
		// Parse YYYY-MM-DD to YYYYMMDD.
		dtstart := strings.ReplaceAll(e.ReleaseDate, "-", "")
		if len(dtstart) < 8 {
			continue
		}
		dtstart = dtstart[:8]

		summary := e.MediaTitle
		if e.EpisodeInfo != "" {
			summary += " - " + e.EpisodeInfo
		}

		uid := e.MediaID
		if e.EpisodeID != "" {
			uid = e.EpisodeID
		}
		uid += "@binge-os-watch"

		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:%s\r\n", uid)
		fmt.Fprintf(&b, "DTSTAMP:%s\r\n", now)
		fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\n", dtstart)
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(summary))
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String(), nil
}

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
