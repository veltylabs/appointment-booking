package time

// Timer represents a cancelable timer.
type Timer interface {
	// Stop prevents the timer from firing. Returns true if the timer was active.
	Stop() bool
}

// Now retrieves the current Unix timestamp in nanoseconds in UTC.
func Now() int64 {
	return provider.UnixNano()
}

// FormatTime formats a value into a time string "HH:MM:SS" applying the timezone offset.
func FormatTime(value any) string {
	return provider.FormatTime(value)
}

// FormatDate formats a value into a date string "YYYY-MM-DD" applying the timezone offset.
func FormatDate(value any) string {
	return provider.FormatDate(value)
}

// FormatDateTime formats a value into a date-time string "YYYY-MM-DD HH:MM:SS" applying the timezone offset.
func FormatDateTime(value any) string {
	return provider.FormatDateTime(value)
}

// FormatDateTimeShort formats a value into a short date-time string "YYYY-MM-DD HH:MM".
func FormatDateTimeShort(value any) string {
	return provider.FormatDateTimeShort(value)
}

// FormatISO8601 formats a UnixNano timestamp into an ISO 8601 string (UTC).
// Format: "YYYY-MM-DDTHH:MM:SSZ"
func FormatISO8601(nano int64) string {
	return provider.FormatISO8601(nano)
}

// FormatCompact formats a UnixNano timestamp into a compact string "YYYYMMDDHHmmss" (UTC).
// Useful for PDF metadata dates, file naming, and compact timestamps.
func FormatCompact(nano int64) string {
	return provider.FormatCompact(nano)
}

// ParseDate parses a date string ("YYYY-MM-DD") into a UnixNano timestamp (UTC).
func ParseDate(dateStr string) (int64, error) {
	return provider.ParseDate(dateStr)
}

// ParseTime parses a time string into minutes since midnight (UTC).
func ParseTime(timeStr string) (int16, error) {
	return provider.ParseTime(timeStr)
}

// ParseDateTime combines date and time strings into a single UnixNano timestamp (UTC).
func ParseDateTime(dateStr, timeStr string) (int64, error) {
	return provider.ParseDateTime(dateStr, timeStr)
}

// Date returns a Unix timestamp in seconds (UTC) for the given year, month, day and optional hour, min, sec.
func Date(y, m, d int, optional ...int) int64 {
	// Simple integer-only Date to Unix seconds converter (UTC)
	// Works for years 1970-2099
	h, min, s := 0, 0, 0
	if len(optional) > 0 {
		h = optional[0]
	}
	if len(optional) > 1 {
		min = optional[1]
	}
	if len(optional) > 2 {
		s = optional[2]
	}

	m = (m + 9) % 12
	y = y - m/10
	days := int64(365*y + y/4 - y/100 + y/400 + (m*306+5)/10 + (d - 1)) - 719468
	return days*86400 + int64(h)*3600 + int64(min)*60 + int64(s)
}

// IsToday checks if the given UnixNano timestamp is today according to the current timezone offset.
func IsToday(nano int64) bool {
	return provider.IsToday(nano)
}

// IsPast checks if the given UnixNano timestamp is in the past.
func IsPast(nano int64) bool {
	return provider.IsPast(nano)
}

// IsFuture checks if the given UnixNano timestamp is in the future.
func IsFuture(nano int64) bool {
	return provider.IsFuture(nano)
}

// DaysBetween calculates the number of full days between two UnixNano timestamps.
func DaysBetween(nano1, nano2 int64) int {
	return DaysBetweenShared(nano1, nano2)
}

// Weekday returns the day of the week (0=Sunday, ..., 6=Saturday) for a Unix timestamp in seconds (UTC).
func Weekday(unixSec int64) int {
	// 1970-01-01 was a Thursday (4)
	return int((unixSec/86400 + 4) % 7)
}

// MidnightUTC returns the Unix timestamp for the midnight UTC of the day containing the given timestamp.
func MidnightUTC(unixSec int64) int64 {
	return (unixSec / 86400) * 86400
}

// LocalMinutesToUnixUTC converts minutes-from-midnight in a local timezone on a given date (Unix UTC sec) to a UTC Unix timestamp.
func LocalMinutesToUnixUTC(dateSec int64, localMinutes int, tz string) int64 {
	return provider.LocalMinutesToUnixUTC(dateSec, localMinutes, tz)
}

// AfterFunc waits for the specified milliseconds then calls f.
func AfterFunc(milliseconds int, f func()) Timer {
	return provider.AfterFunc(milliseconds, f)
}

// Internal interface for the singleton provider
type timeProvider interface {
	UnixNano() int64
	FormatDate(value any) string
	FormatTime(value any) string
	FormatDateTime(value any) string
	FormatDateTimeShort(value any) string
	FormatISO8601(nano int64) string
	FormatCompact(nano int64) string
	ParseDate(dateStr string) (int64, error)
	ParseTime(timeStr string) (int16, error)
	ParseDateTime(dateStr, timeStr string) (int64, error)
	IsToday(nano int64) bool
	IsPast(nano int64) bool
	IsFuture(nano int64) bool
	LocalMinutesToUnixUTC(dateSec int64, localMinutes int, tz string) int64
	AfterFunc(milliseconds int, f func()) Timer
}

var provider timeProvider
