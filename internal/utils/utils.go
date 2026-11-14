package utils

import (
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

const (
	ApproxDaysInYear  = 365
	ApproxDaysInMonth = 28
	DaysInWeek        = 7
)

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TimeElapsed(then time.Time) string {
	var parts []string
	var text string

	now := time.Now()
	diff := now.Sub(then)
	day := math.Round(diff.Hours() / 24)
	year := math.Round(day / ApproxDaysInYear)
	month := math.Round(day / ApproxDaysInMonth)
	week := math.Round(day / DaysInWeek)
	hour := math.Round(math.Abs(diff.Hours()))
	minute := math.Round(math.Abs(diff.Minutes()))
	second := math.Round(math.Abs(diff.Seconds()))

	if year > 0 {
		parts = append(parts, strconv.Itoa(int(year))+"y")
	}

	if month > 0 {
		parts = append(parts, strconv.Itoa(int(month))+"mo")
	}

	if week > 0 {
		parts = append(parts, strconv.Itoa(int(week))+"w")
	}

	if day > 0 {
		parts = append(parts, strconv.Itoa(int(day))+"d")
	}

	if hour > 0 {
		parts = append(parts, strconv.Itoa(int(hour))+"h")
	}

	if minute > 0 {
		parts = append(parts, strconv.Itoa(int(minute))+"m")
	}

	if second > 0 {
		parts = append(parts, strconv.Itoa(int(second))+"s")
	}

	if len(parts) == 0 {
		return "now"
	}

	return parts[0] + text
}

func BoolPtr(b bool) *bool { return &b }

func StringPtr(s string) *string { return &s }

func UintPtr(u uint) *uint { return &u }

func IntPtr(u int) *int { return &u }

func ShortNumber(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}

	if n < 1000000 {
		return strconv.Itoa(n/1000) + "k"
	}

	return strconv.Itoa(n/1000000) + "m"
}

// GetStylePrefix extracts ANSI codes from a lipgloss style without the trailing reset.
// This allows styled text to be concatenated without breaking parent background colors.
func GetStylePrefix(s lipgloss.Style) string {
	rendered := s.Render("")
	// Strip trailing SGR reset sequence: \x1b[0m (4 bytes) or \x1b[m (3 bytes).
	// lipgloss v2 uses the shorter form (\x1b[m); both are valid per ANSI spec.
	if len(rendered) >= 4 && rendered[len(rendered)-4:] == "\x1b[0m" {
		return rendered[:len(rendered)-4]
	}
	if len(rendered) >= 3 && rendered[len(rendered)-3:] == "\x1b[m" {
		return rendered[:len(rendered)-3]
	}
	return rendered
}

func Clamp(mn, wanted, mx int) int {
	return min(mx, max(mn, wanted))
}

// Remove all ANSI reset codes
func RemoveReset(s string) string {
	return strings.ReplaceAll(s, "\x1b[m", "")
}

// Remove last ANSI reset code
func RemoveLastReset(s string) string {
	idx := strings.LastIndex(s, "\x1b[m")
	if idx < 0 {
		return s
	}

	return s[:idx]
}

// IsNewerVersion returns true if latest is a newer semantic version than current.
// If current is "dev", "(devel)", or empty, any valid latest version is considered newer.
func IsNewerVersion(latest, current string) bool {
	if latest == "" {
		return false
	}
	cleanLatest := strings.TrimPrefix(strings.TrimSpace(latest), "v")
	cleanCurrent := strings.TrimPrefix(strings.TrimSpace(current), "v")

	if cleanCurrent == "dev" || cleanCurrent == "" || cleanCurrent == "(devel)" {
		return true
	}

	if cleanLatest == cleanCurrent {
		return false
	}

	latestParts := strings.Split(cleanLatest, ".")
	currentParts := strings.Split(cleanCurrent, ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		lNum, lErr := strconv.Atoi(latestParts[i])
		cNum, cErr := strconv.Atoi(currentParts[i])
		if lErr == nil && cErr == nil {
			if lNum > cNum {
				return true
			}
			if lNum < cNum {
				return false
			}
		} else {
			if latestParts[i] != currentParts[i] {
				return latestParts[i] > currentParts[i]
			}
		}
	}

	return len(latestParts) > len(currentParts)
}
