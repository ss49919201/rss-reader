package rss

import "time"

var jst = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}()

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(jst).Format("2006-01-02 15:04") + " JST"
}
