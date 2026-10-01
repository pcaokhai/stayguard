package pricing

import "time"

const minutesPerHour = 60

// blocks counts billable hours: every full hour, plus one when the remainder passes the grace.
func blocks(minutes int64, grace int) int64 {
	full, rem := minutes/minutesPerHour, minutes%minutesPerHour
	if rem > int64(grace) {
		full++
	}
	return full
}

// minutesBetween truncates seconds; a negative span counts as zero because every caller only
// bills time that lies beyond a boundary.
func minutesBetween(from, to time.Time) int64 {
	if !to.After(from) {
		return 0
	}
	return int64(to.Sub(from) / time.Minute)
}

// wallAt builds a wall-clock instant in loc. Using time.Date with a day offset (not +24h)
// keeps the clock time right across a zone offset change.
func wallAt(loc *time.Location, day time.Time, dayOffset int, c Clock) time.Time {
	y, m, d := day.Date()
	return time.Date(y, m, d+dayOffset, c.Hour, c.Minute, 0, 0, loc)
}

func (c Clock) minuteOfDay() int { return c.Hour*minutesPerHour + c.Minute }
