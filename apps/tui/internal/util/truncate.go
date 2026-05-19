package util

import (
	"strconv"
	"time"
)

func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func TruncateStart(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[len(s)-maxLen:]
	}
	return "..." + s[len(s)-(maxLen-3):]
}

func TruncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 7 {
		return s[:maxLen]
	}
	half := (maxLen - 3) / 2
	return s[:half] + "..." + s[len(s)-(maxLen-half-3):]
}

func SafeWidth(w, min int) int {
	if w < min {
		return min
	}
	return w
}

func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

func FormatDurationShort(d time.Duration) string {
	total := int(d.Seconds())
	if total < 60 {
		return d.Round(time.Second).String()
	}
	minutes := total / 60
	seconds := total % 60
	return formatTimeUnit(minutes, "m") + formatTimeUnit(seconds, "s")
}

func formatTimeUnit(v int, unit string) string {
	if v == 0 {
		return ""
	}
	return formatInt(v) + unit
}

func formatInt(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
