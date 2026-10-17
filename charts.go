package boxmaid

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Quadrant charts and XY charts.

var (
	quadPoint = regexp.MustCompile(`^(.+?)\s*:\s*\[\s*([\d.]+)\s*,\s*([\d.]+)\s*\]`)
	axisRange = regexp.MustCompile(`^(?:"([^"]*)"|(\S+(?:\s+\S+)*?))?\s*(-?[\d.]+)\s*-->\s*(-?[\d.]+)\s*$`)
	axisList  = regexp.MustCompile(`^(?:"([^"]*)"|([^\[]*?))?\s*\[(.*)\]\s*$`)
	seriesRe  = regexp.MustCompile(`^(bar|line)\s*(?:"[^"]*"\s*)?\[(.*)\]\s*$`)
)

// renderQuadrant draws a quadrant chart: a box split in four with each
// quadrant's label, the points where they fall, and the axes' ends.
func renderQuadrant(_, rest string, opt Options) (*Picture, string, error) {
	type point struct {
		name string
		x, y float64
	}
	var title string
	var xAxis, yAxis [2]string
	quads := map[int]string{}
	var points []point
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		word, arg, _ := strings.Cut(line, " ")
		arg = strings.TrimSpace(arg)
		switch {
		case line == "":
		case word == "title":
			title = arg
		case word == "x-axis" || word == "y-axis":
			lo, hi, _ := strings.Cut(arg, "-->")
			ends := [2]string{strings.TrimSpace(lo), strings.TrimSpace(hi)}
			if word == "x-axis" {
				xAxis = ends
			} else {
				yAxis = ends
			}
		case strings.HasPrefix(word, "quadrant-"):
			n, _ := strconv.Atoi(strings.TrimPrefix(word, "quadrant-"))
			quads[n] = arg
		default:
			if m := quadPoint.FindStringSubmatch(line); m != nil {
				x, _ := strconv.ParseFloat(m[2], 64)
				y, _ := strconv.ParseFloat(m[3], 64)
				points = append(points, point{strings.Trim(m[1], `"`), math.Max(0, math.Min(1, x)), math.Max(0, math.Min(1, y))})
			}
		}
	}
	if len(points) == 0 && len(quads) == 0 {
		return nil, title, ErrEmpty
	}
	W := 64
	if opt.Width > 0 {
		W = max(30, min(W, opt.Width-2))
	}
	H := max(12, W*2/5) | 1 // odd, so the middle line is a row
	left := 0
	if yAxis[0] != "" || yAxis[1] != "" {
		left = 2
	}
	cv := newCanvas(left+W+20, H+3, opt.ASCII)
	x0 := left
	cv.box(x0, 0, W, H, lineSolid, false, false, RoleCluster, "")
	midX, midY := x0+W/2, H/2
	cv.vline(midX, 0, H-1, lineDotted, RoleCluster, "")
	cv.hline(x0, x0+W-1, midY, lineDotted, RoleCluster, "")
	// Mermaid numbers quadrants like the maths does: 1 top right, 2 top
	// left, 3 bottom left, 4 bottom right.
	put := func(q, x1, x2, y int) {
		if t := quads[q]; t != "" {
			t = truncate(t, x2-x1-1)
			cv.text(x1+(x2-x1-width(t))/2, y, t, RoleClusterTitle, "", true)
		}
	}
	put(1, midX, x0+W-1, 1)
	put(2, x0, midX, 1)
	put(3, x0, midX, midY+1)
	put(4, midX, x0+W-1, midY+1)
	for _, p := range points {
		x := x0 + 1 + int(p.x*float64(W-3)+0.5)
		y := 1 + int((1-p.y)*float64(H-3)+0.5)
		cv.glyphAt(x, y, "●", RoleBar, "")
		name := p.name
		if x+2+width(name) < x0+W-1 {
			cv.text(x+2, y, name, RoleNodeText, "", false)
		} else {
			cv.text(x-1-width(name), y, name, RoleNodeText, "", false)
		}
	}
	if xAxis[0] != "" {
		cv.text(x0, H, xAxis[0], RoleMuted, "", false)
	}
	if xAxis[1] != "" {
		cv.text(x0+W-width(xAxis[1]), H, xAxis[1], RoleMuted, "", false)
	}
	if yAxis[1] != "" {
		cv.text(x0+W+1, 0, "↑ "+yAxis[1], RoleMuted, "", false)
	}
	if yAxis[0] != "" {
		cv.text(x0+W+1, H-1, "↓ "+yAxis[0], RoleMuted, "", false)
	}
	return cv.picture(), title, nil
}

// renderXYChart draws bar and line series over categories or a numeric x
// axis; "xychart-beta horizontal" lays the bars sideways.
func renderXYChart(body, rest string, opt Options) (*Picture, string, error) {
	horizontal := strings.Contains(strings.SplitN(strings.TrimSpace(body), "\n", 2)[0], "horizontal")
	var title, xTitle, yTitle string
	var cats []string
	var yMin, yMax float64
	yRange := false
	type series struct {
		bar  bool
		vals []float64
	}
	var all []series
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		word, arg, _ := strings.Cut(line, " ")
		arg = strings.TrimSpace(arg)
		switch word {
		case "title":
			title = strings.Trim(arg, `"`)
		case "x-axis":
			if m := axisList.FindStringSubmatch(arg); m != nil {
				xTitle = strings.TrimSpace(m[1] + m[2])
				for _, c := range splitTopLevel(m[3], ',') {
					cats = append(cats, strings.Trim(strings.TrimSpace(c), `"`))
				}
			} else if m := axisRange.FindStringSubmatch(arg); m != nil {
				xTitle = strings.TrimSpace(m[1] + m[2])
			}
		case "y-axis":
			if m := axisRange.FindStringSubmatch(arg); m != nil {
				yTitle = strings.TrimSpace(m[1] + m[2])
				yMin, _ = strconv.ParseFloat(m[3], 64)
				yMax, _ = strconv.ParseFloat(m[4], 64)
				yRange = true
			} else {
				yTitle = strings.Trim(arg, `"`)
			}
		default:
			if m := seriesRe.FindStringSubmatch(line); m != nil {
				s := series{bar: m[1] == "bar"}
				for _, v := range strings.Split(m[2], ",") {
					f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
					if err == nil {
						s.vals = append(s.vals, f)
					}
				}
				all = append(all, s)
			}
		}
	}
	n := 0
	for _, s := range all {
		n = max(n, len(s.vals))
	}
	if n == 0 {
		return nil, title, ErrEmpty
	}
	for len(cats) < n {
		cats = append(cats, strconv.Itoa(len(cats)+1))
	}
	if !yRange {
		yMin, yMax = 0, 0
		for _, s := range all {
			for _, v := range s.vals {
				yMin, yMax = math.Min(yMin, v), math.Max(yMax, v)
			}
		}
		if yMax == yMin {
			yMax = yMin + 1
		}
	}
	if horizontal {
		return xyHorizontal(title, cats, all[0].vals, yMin, yMax, opt), title, nil
	}
	_ = xTitle
	// Vertical: a value axis on the left, a column per category.
	H := 12
	nBars := 0
	for _, s := range all {
		if s.bar {
			nBars++
		}
	}
	colW := max(4, max(nBars*2, maxWidth(cats)+1))
	if opt.Width > 0 {
		colW = max(2, min(colW, (opt.Width-10)/n))
	}
	fmtV := formatNumber
	axisW := max(width(fmtV(yMax)), width(fmtV(yMin))) + 1
	x0 := axisW + 1
	cv := newCanvas(x0+n*colW+2, H+4, opt.ASCII)
	// The axis with three values.
	cv.vline(x0-1, 0, H, lineSolid, RoleMuted, "")
	cv.hline(x0-1, x0+n*colW, H, lineSolid, RoleMuted, "")
	for _, f := range []float64{0, 0.5, 1} {
		v := yMin + f*(yMax-yMin)
		y := H - 1 - int(f*float64(H-1)+0.5)
		lbl := fmtV(v)
		cv.text(axisW-width(lbl), y, lbl, RoleMuted, "", false)
	}
	if yTitle != "" {
		cv.text(0, H+2, yTitle, RoleMuted, "", false)
	}
	eighths := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	if opt.ASCII {
		eighths = []string{" ", "#", "#", "#", "#", "#", "#", "#", "#"}
	}
	height := func(v float64) float64 { // in rows, fractional
		return math.Max(0, math.Min(1, (v-yMin)/(yMax-yMin))) * float64(H)
	}
	bi := 0
	for _, s := range all {
		if !s.bar {
			continue
		}
		barW := max(1, (colW-1)/max(1, nBars))
		for i, v := range s.vals {
			x := x0 + i*colW + bi*barW
			units := int(height(v)*8 + 0.5)
			for row := 0; units > 0; row++ {
				part := min(8, units)
				cv.text(x, H-1-row, strings.Repeat(eighths[part], barW), RoleBar, gitColors[bi%len(gitColors)], false)
				units -= part
			}
		}
		bi++
	}
	for li, s := range all {
		if s.bar {
			continue
		}
		var prev [2]int
		for i, v := range s.vals {
			x := x0 + i*colW + colW/2
			y := H - 1 - int(height(v)*float64(H-1)/float64(H)+0.5)
			if i > 0 {
				drawDots(cv, prev, [2]int{x, y})
			}
			cv.text(x, y, "●", RoleAccent, gitColors[(li+3)%len(gitColors)], true) // on top of the bars
			prev = [2]int{x, y}
		}
	}
	for i, c := range cats {
		cv.text(x0+i*colW, H+1, truncate(c, colW-1), RoleMuted, "", false)
	}
	return cv.picture(), title, nil
}

// drawDots joins two points of a line series with dots.
func drawDots(cv *canvas, a, b [2]int) {
	steps := max(abs(b[0]-a[0]), abs(b[1]-a[1]))
	for s := 1; s < steps; s++ {
		x := a[0] + (b[0]-a[0])*s/steps
		y := a[1] + (b[1]-a[1])*s/steps
		if c := cv.at(x, y); c != nil && c.text == "" && c.glyph == "" {
			cv.glyphAt(x, y, "·", RoleAccent, "")
		}
	}
}

// xyHorizontal draws one bar series sideways: a row per category.
func xyHorizontal(title string, cats []string, vals []float64, lo, hi float64, opt Options) *Picture {
	nameW := maxWidth(cats)
	W := 50
	if opt.Width > 0 {
		W = max(10, min(60, opt.Width-nameW-12))
	}
	cv := newCanvas(nameW+W+14, len(cats)+1, opt.ASCII)
	eighths := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	for i, c := range cats {
		cv.text(0, i, c, RoleNodeText, "", false)
		if i >= len(vals) {
			continue
		}
		units := int(math.Max(0, math.Min(1, (vals[i]-lo)/(hi-lo)))*float64(W)*8 + 0.5)
		bar := strings.Repeat("█", units/8) + eighths[units%8]
		if opt.ASCII {
			bar = strings.Repeat("#", (units+4)/8)
		}
		cv.text(nameW+2, i, bar, RoleBar, gitColors[0], false)
		cv.text(nameW+3+width(bar), i, formatNumber(vals[i]), RoleMuted, "", false)
	}
	_ = title
	return cv.picture()
}
