package boxmaid

import (
	"regexp"
	"strconv"
	"strings"
)

// Gantt charts: one row per task under its section, a bar from start to
// end on a shared time axis. Done tasks are muted, active ones accented,
// critical ones in the warning colour; milestones are a ◆. Starts may be
// dates or "after" other tasks, ends dates, durations (3d, 1w, 12h) or
// "until" another task; "excludes weekends" stretches durations over them.

type ganttTask struct {
	name, id     string
	section      string
	done, active bool
	crit, mile   bool
	start, end   int64 // milliseconds since 1970, UTC
	startSpec    string
	endSpec      string
	resolved     bool
}

var (
	ganttTask1 = regexp.MustCompile(`^(.+?)\s*:\s*(.*)$`)
	ganttDur   = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(ms|s|m|h|d|w|M|y)$`)
	ganttAfter = regexp.MustCompile(`^after\s+(.+)$`)
	ganttUntil = regexp.MustCompile(`^until\s+(.+)$`)
)

func parseGanttDate(s, format string) (int64, bool) {
	s = strings.TrimSpace(s)
	for _, f := range []string{format, "YYYY-MM-DD", "YYYY-MM-DD HH:mm", "YYYY-MM-DDTHH:mm:ss"} {
		if t, ok := parseMomentDate(s, f); ok {
			return t, true
		}
	}
	return 0, false
}

// addDuration adds a Mermaid duration ("3d", "1w", "12h") to t, skipping
// weekends for day-based durations when asked.
func addDuration(t int64, spec string, skipWeekends bool) (int64, bool) {
	m := ganttDur.FindStringSubmatch(strings.TrimSpace(spec))
	if m == nil {
		return t, false
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	var unit int64
	switch m[2] {
	case "ms":
		unit = 1
	case "s":
		unit = msSecond
	case "m":
		unit = msMinute
	case "h":
		unit = msHour
	case "d":
		unit = msDay
	case "w":
		unit = 7 * msDay
	case "M":
		return addMonths(t, int64(v)), true
	case "y":
		return addMonths(t, 12*int64(v)), true
	}
	if skipWeekends && (m[2] == "d" || m[2] == "w") {
		days := int64(v)
		if m[2] == "w" {
			days *= 5
		}
		for days > 0 {
			t += msDay
			if wd := weekday(t); wd != 0 && wd != 6 {
				days--
			}
		}
		return t, true
	}
	return t + int64(v*float64(unit)), true
}

func renderGantt(_, rest string, opt Options) (*Picture, string, error) {
	layout := "YYYY-MM-DD"
	title, section := "", ""
	skipWeekends := false
	var tasks []*ganttTask
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		word, arg, _ := strings.Cut(line, " ")
		arg = strings.TrimSpace(arg)
		switch word {
		case "":
			continue
		case "title":
			title = arg
			continue
		case "dateFormat":
			layout = arg
			continue
		case "section":
			section = arg
			continue
		case "excludes":
			skipWeekends = strings.Contains(strings.ToLower(arg), "weekend")
			continue
		case "axisFormat", "tickInterval", "todayMarker", "includes", "weekday", "inclusiveEndDates", "topAxis",
			"accTitle", "accTitle:", "accDescr", "accDescr:", "click", "displayMode":
			continue
		}
		m := ganttTask1.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		t := &ganttTask{name: m[1], section: section}
		var meta []string
		for _, p := range strings.Split(m[2], ",") {
			p = strings.TrimSpace(p)
			switch p {
			case "done":
				t.done = true
			case "active":
				t.active = true
			case "crit":
				t.crit = true
			case "milestone":
				t.mile = true
			case "":
			default:
				meta = append(meta, p)
			}
		}
		switch len(meta) {
		case 1:
			t.endSpec = meta[0]
		case 2:
			t.startSpec, t.endSpec = meta[0], meta[1]
		default:
			if len(meta) >= 3 {
				t.id, t.startSpec, t.endSpec = meta[0], meta[1], meta[2]
			}
		}
		tasks = append(tasks, t)
	}
	if len(tasks) == 0 {
		return nil, title, ErrEmpty
	}
	byID := map[string]*ganttTask{}
	for _, t := range tasks {
		if t.id != "" {
			byID[t.id] = t
		}
	}
	// Resolve starts and ends in order; "after" may refer forward, so
	// repeat until nothing changes.
	for range len(tasks) + 1 {
		var prev *ganttTask
		for _, t := range tasks {
			if !t.resolved {
				resolveTask(t, prev, byID, layout, skipWeekends)
			}
			prev = t
		}
	}
	var lo, hi int64
	found := false
	for _, t := range tasks {
		if !t.resolved {
			continue
		}
		if !found || t.start < lo {
			lo = t.start
		}
		if !found || t.end > hi {
			hi = t.end
		}
		found = true
	}
	if !found {
		return nil, title, ErrEmpty
	}
	if hi <= lo {
		hi = lo + msDay
	}
	// Columns: task names, then the chart.
	W := opt.Width
	if W <= 0 {
		W = 100
	}
	nameW := 8
	for _, t := range tasks {
		nameW = max(nameW, min(28, width(t.name)))
	}
	chartW := max(20, W-nameW-3)
	span := hi - lo
	col := func(t int64) int { return int(float64(chartW-1) * float64(t-lo) / float64(span)) }
	rows := 2 // axis
	sections := 0
	last := "\x00"
	for _, t := range tasks {
		if t.section != last {
			sections++
			last = t.section
		}
		rows++
	}
	cv := newCanvas(nameW+3+chartW, rows+sections+1, opt.ASCII)
	x0 := nameW + 3
	// Axis: dates at even steps, as many as fit.
	cv.hline(x0, x0+chartW-1, 1, lineSolid, RoleMuted, "")
	dayFmt := "day"
	if span < 2*msDay {
		dayFmt = "time"
	} else if span > 400*msDay {
		dayFmt = "month"
	}
	labelW := width(formatAxisDate(lo, dayFmt)) + 3
	ticks := max(1, chartW/labelW)
	free := 0 // the first column a label may start at
	for i := 0; i <= ticks; i++ {
		x := x0 + i*(chartW-1)/ticks
		lbl := formatAxisDate(lo+int64(float64(span)*float64(i)/float64(ticks)), dayFmt)
		lx := min(x, x0+chartW-width(lbl))
		if lx < free {
			continue // it would run into the previous date
		}
		cv.text(lx, 0, lbl, RoleMuted, "", false)
		cv.glyphAt(x, 1, "┬", RoleMuted, "")
		free = lx + width(lbl) + 2
	}
	y := 2
	last = "\x00"
	for _, t := range tasks {
		if t.section != last {
			last = t.section
			if t.section != "" {
				cv.text(0, y, truncate(t.section, nameW+2), RoleClusterTitle, "", true)
				y++
			}
		}
		cv.text(1, y, truncate(t.name, nameW), RoleNodeText, "", false)
		if !t.resolved {
			cv.text(x0, y, "?", RoleMuted, "", false)
			y++
			continue
		}
		role := RoleBar
		bar := "█"
		switch {
		case t.crit:
			role = RoleWarn
		case t.active:
			role = RoleAccent
		case t.done:
			role, bar = RoleMuted, "▓"
		}
		a, b := col(t.start), col(t.end)
		if t.mile || t.end <= t.start {
			cv.text(x0+a, y, "◆", role, "", false)
		} else {
			cv.text(x0+a, y, strings.Repeat(bar, max(1, b-a)), role, "", false)
		}
		y++
	}
	return cv.picture(), title, nil
}

func resolveTask(t, prev *ganttTask, byID map[string]*ganttTask, layout string, skip bool) {
	// Start: a date, "after a b" (the latest end), or right after the
	// previous task.
	var start int64
	switch {
	case t.startSpec == "":
		if prev == nil || !prev.resolved {
			return
		}
		start = prev.end
	default:
		if m := ganttAfter.FindStringSubmatch(t.startSpec); m != nil {
			start = -1 << 62 // the latest end of the tasks named
			for _, id := range strings.Fields(m[1]) {
				o, ok := byID[id]
				if !ok || !o.resolved {
					return
				}
				if o.end > start {
					start = o.end
				}
			}
		} else if d, ok := parseGanttDate(t.startSpec, layout); ok {
			start = d
		} else {
			return
		}
	}
	// End: a date, a duration, or "until a" (that task's start).
	end := start
	if m := ganttUntil.FindStringSubmatch(t.endSpec); m != nil {
		o, ok := byID[strings.TrimSpace(m[1])]
		if !ok || !o.resolved {
			return
		}
		end = o.start
	} else if e, ok := addDuration(start, t.endSpec, skip); ok {
		end = e
	} else if d, ok := parseGanttDate(t.endSpec, layout); ok {
		end = d
	}
	t.start, t.end, t.resolved = start, end, true
}
