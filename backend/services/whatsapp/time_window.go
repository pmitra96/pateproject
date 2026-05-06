package whatsapp

import "time"

func dayWindow(t time.Time) (time.Time, time.Time) {
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return start, start.Add(24 * time.Hour)
}
