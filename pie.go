package boxmaid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var pieSlice = regexp.MustCompile(`^"([^"]*)"\s*:\s*([0-9.]+)\s*$`)

// renderPie draws a pie chart as horizontal bars, largest first.
func renderPie(src string, opt Options) (*Picture, string, error) {
	type slice struct {
		name string
		v    float64
	}
	var slices []slice
	title := ""
	showData := false
	total := 0.0
	for i, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if i == 0 || strings.HasPrefix(line, "pie") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "pie"))
			if strings.HasPrefix(line, "showData") {
				showData = true
				line = strings.TrimSpace(strings.TrimPrefix(line, "showData"))
			}
		}
		if strings.HasPrefix(line, "title") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "title"))
			continue
		}
		if m := pieSlice.FindStringSubmatch(line); m != nil {
			v, err := strconv.ParseFloat(m[2], 64)
			if err == nil && v > 0 {
				slices = append(slices, slice{m[1], v})
				total += v
			}
		}
	}
	if len(slices) == 0 {
		return nil, title, ErrEmpty
	}
	for i := 1; i < len(slices); i++ {
		for j := i; j > 0 && slices[j].v > slices[j-1].v; j-- {
			slices[j], slices[j-1] = slices[j-1], slices[j]
		}
	}
	nameW := 0
	for _, s := range slices {
		nameW = max(nameW, min(30, width(s.name)))
	}
	barW := 40
	if opt.Width > 0 {
		barW = max(10, min(40, opt.Width-nameW-20))
	}
	cv := newCanvas(nameW+barW+24, len(slices), opt.ASCII)
	for i, s := range slices {
		name := s.name
		if width(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		cv.text(0, i, name, RoleNodeText, "", false)
		frac := s.v / total
		n := int(frac*float64(barW) + 0.5)
		bar := strings.Repeat("█", n)
		if opt.ASCII {
			bar = strings.Repeat("#", n)
		}
		cv.text(nameW+2, i, bar, RoleBar, "", false)
		info := fmt.Sprintf("%.1f%%", frac*100)
		if showData {
			info += "  (" + formatNumber(s.v) + ")"
		}
		cv.text(nameW+3+n, i, info, RoleMuted, "", false)
	}
	return cv.picture(), title, nil
}
