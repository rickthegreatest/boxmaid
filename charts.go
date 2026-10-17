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

// xySeries is one bar or line series of an XY chart.
type xySeries struct {
	bar  bool
	vals []float64
}

// renderXYChart draws bar and line series over categories or a numeric x
// axis; "xychart-beta horizontal" lays the bars sideways.
func renderXYChart(body, rest string, opt Options) (*Picture, string, error) {
	horizontal := strings.Contains(strings.SplitN(strings.TrimSpace(body), "\n", 2)[0], "horizontal")
	var title, xTitle, yTitle string
	var cats []string
	var yMin, yMax float64
	yRange := false
	var all []xySeries
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
				s := xySeries{bar: m[1] == "bar"}
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
		return xyHorizontal(cats, all, xTitle, yTitle, yMin, yMax, opt), title, nil
	}
	// Vertical: a value axis on the left, a column per category. The y
	// title heads the value axis; the x title sits under the categories.
	H := 12
	top := 0
	if yTitle != "" {
		top = 1
	}
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
	rows := top + H + 2
	if xTitle != "" {
		rows++
	}
	cv := newCanvas(max(x0+n*colW+2, width(yTitle)), rows, opt.ASCII)
	if yTitle != "" {
		cv.text(0, 0, yTitle, RoleMuted, "", false)
	}
	// The axis with three values.
	cv.vline(x0-1, top, top+H, lineSolid, RoleMuted, "")
	cv.hline(x0-1, x0+n*colW, top+H, lineSolid, RoleMuted, "")
	for _, f := range []float64{0, 0.5, 1} {
		v := yMin + f*(yMax-yMin)
		y := H - 1 - int(f*float64(H-1)+0.5)
		lbl := fmtV(v)
		cv.text(axisW-width(lbl), top+y, lbl, RoleMuted, "", false)
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
				cv.text(x, top+H-1-row, strings.Repeat(eighths[part], barW), RoleBar, gitColors[bi%len(gitColors)], false)
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
			y := top + H - 1 - int(height(v)*float64(H-1)/float64(H)+0.5)
			if i > 0 {
				drawDots(cv, prev, [2]int{x, y})
			}
			cv.text(x, y, "●", RoleAccent, gitColors[(li+3)%len(gitColors)], true) // on top of the bars
			prev = [2]int{x, y}
		}
	}
	for i, c := range cats {
		cv.text(x0+i*colW, top+H+1, truncate(c, colW-1), RoleMuted, "", false)
	}
	if xTitle != "" {
		t := truncate(xTitle, n*colW)
		cv.text(x0+(n*colW-width(t))/2, top+H+2, t, RoleMuted, "", false)
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

// xyHorizontal draws the series sideways: a block of rows per category,
// one row for each bar series, with line series as points joined across
// the blocks. The x title heads the category names; the y title sits under
// the bars.
func xyHorizontal(cats []string, all []xySeries, xTitle, yTitle string, lo, hi float64, opt Options) *Picture {
	nameW := max(maxWidth(cats), width(xTitle))
	W := 50
	if opt.Width > 0 {
		W = max(10, min(60, opt.Width-nameW-12))
	}
	nBars := 0
	for _, s := range all {
		if s.bar {
			nBars++
		}
	}
	k := max(1, nBars)
	gap := 0
	if k > 1 {
		gap = 1
	}
	top := 0
	if xTitle != "" {
		top = 1
	}
	rows := top + len(cats)*(k+gap) - gap + 1
	if yTitle != "" {
		rows++
	}
	cv := newCanvas(nameW+W+14, rows, opt.ASCII)
	if xTitle != "" {
		cv.text(0, 0, xTitle, RoleMuted, "", false)
	}
	block := func(i int) int { return top + i*(k+gap) }
	frac := func(v float64) float64 { return math.Max(0, math.Min(1, (v-lo)/(hi-lo))) }
	eighths := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	for i, c := range cats {
		cv.text(0, block(i), c, RoleNodeText, "", false)
	}
	bi := 0
	for _, s := range all {
		if !s.bar {
			continue
		}
		for i, v := range s.vals {
			if i >= len(cats) {
				break
			}
			y := block(i) + bi
			units := int(frac(v)*float64(W)*8 + 0.5)
			bar := strings.Repeat("█", units/8) + eighths[units%8]
			if opt.ASCII {
				bar = strings.Repeat("#", (units+4)/8)
			}
			cv.text(nameW+2, y, bar, RoleBar, gitColors[bi%len(gitColors)], false)
			cv.text(nameW+3+width(bar), y, formatNumber(v), RoleMuted, "", false)
		}
		bi++
	}
	for li, s := range all {
		if s.bar {
			continue
		}
		var prev [2]int
		for i, v := range s.vals {
			if i >= len(cats) {
				break
			}
			x := nameW + 2 + min(W-1, int(frac(v)*float64(W-1)+0.5))
			y := block(i) + (k-1)/2
			if i > 0 {
				drawDots(cv, prev, [2]int{x, y})
			}
			cv.text(x, y, "●", RoleAccent, gitColors[(li+3)%len(gitColors)], true)
			prev = [2]int{x, y}
		}
	}
	if yTitle != "" {
		t := truncate(yTitle, W)
		cv.text(nameW+2+(W-width(t))/2, rows-1, t, RoleMuted, "", false)
	}
	return cv.picture()
}
