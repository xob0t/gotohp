package backend

import (
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// filenamePattern holds a compiled regex and whether it captures a full time component.
type filenamePattern struct {
	re       *regexp.Regexp
	hasTime  bool
	isUnixMs bool
}

var filenameTimestampPatterns = []filenamePattern{
	// YYYYMMDD[_-]HHMMSS — e.g. 20240709_182027.mp4, PXL_20231123_182518628.jpg
	{regexp.MustCompile(`(\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})\d*`), true, false},
	// YYYY-MM-DD[sep HHMMSS] — e.g. 2022-10-24-150226287.mp4, Screenshot 2026-02-13 093505.png
	{regexp.MustCompile(`(\d{4})-(\d{1,2})-(\d{1,2})(?:[ _-](\d{2})(\d{2})(\d{2})\d*)?`), false, false},
	// [non-digit]YYYYMMDDHHMMSS[non-digit] — e.g. lv_7324034615860006160_20240617193045.mp4
	{regexp.MustCompile(`(?:^|[^0-9])(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})(?:[^0-9]|$)`), true, false},
	// Unix milliseconds — e.g. FaceApp_1658848332262.jpg (covers 2001–2033)
	{regexp.MustCompile(`(?:^|[^0-9])(1\d{12})(?:[^0-9]|$)`), true, true},
}

// parseTimestampFromFilename tries each pattern in priority order.
// Returns the extracted time and true on success.
func parseTimestampFromFilename(filename string) (time.Time, bool) {
	base := filepath.Base(filename)

	for _, pat := range filenameTimestampPatterns {
		m := pat.re.FindStringSubmatch(base)
		if m == nil {
			continue
		}

		if pat.isUnixMs {
			ms, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil {
				continue
			}
			t := time.UnixMilli(ms).In(time.Local)
			if t.Year() < 1990 || t.Year() > time.Now().Year()+1 {
				continue
			}
			return t, true
		}

		year, month, day := m[1], m[2], m[3]
		hour, min, sec := "12", "00", "00"

		if pat.hasTime {
			hour, min, sec = m[4], m[5], m[6]
		} else if len(m) >= 7 && m[4] != "" {
			hour, min, sec = m[4], m[5], m[6]
		}

		pad2 := func(s string) string {
			if len(s) < 2 {
				return "0" + s
			}
			return s
		}

		t, err := time.ParseInLocation("20060102 150405",
			year+pad2(month)+pad2(day)+" "+pad2(hour)+min+sec, time.Local)
		if err != nil {
			continue
		}
		if t.Year() < 1990 || t.Year() > time.Now().Year()+1 {
			continue
		}
		return t, true
	}

	return time.Time{}, false
}
