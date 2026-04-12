package repository

import "time"

func toUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnix(ts int64) time.Time {
	return time.Unix(ts, 0).UTC()
}
