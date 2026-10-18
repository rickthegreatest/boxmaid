package renderer

import "strings"

// Timelines: periods down a spine, each with its events beside it, grouped
// under their sections.
//
//	2002 ─┬─ LinkedIn
//	      │
//	2004 ─┼─ Facebook
//	      │  Google

func renderTimeline(_, rest string, opt Options) (*Picture, string, error) {
	type period struct {
		name, section string
		events        []string
	}
	title, section := "", ""
	var periods []*period
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		word, arg, _ := strings.Cut(line, " ")
		switch word {
		case "":
			continue
		case "title":
			title = strings.TrimSpace(arg)
			continue
		case "section":
			section = strings.TrimSpace(arg)
			continue
		case "accTitle", "accTitle:", "accDescr", "accDescr:":
			continue
		}
		parts := strings.Split(line, ":")
		head := strings.TrimSpace(parts[0])
		var events []string
		for _, e := range parts[1:] {
			if e = strings.TrimSpace(e); e != "" {
				events = append(events, e)
			}
		}
		if head == "" && len(periods) > 0 {
			// ": another event" continues the period above.
			p := periods[len(periods)-1]
			p.events = append(p.events, events...)
			continue
		}
		periods = append(periods, &period{head, section, events})
	}
	if len(periods) == 0 {
		return nil, title, ErrEmpty
	}
	nameW := 0
	for _, p := range periods {
		nameW = max(nameW, width(p.name))
	}
	evW := 60
	if opt.Width > 0 {
		evW = max(16, opt.Width-nameW-5)
	}
	var lines int
	last := "\x00"
	for _, p := range periods {
		if p.section != last {
			lines += 2
			last = p.section
		}
		for _, e := range p.events {
			lines += len(wrap([]string{e}, evW))
		}
		lines += 2
	}
	cv := newCanvas(nameW+evW+6, lines+2, opt.ASCII)
	spine := nameW + 2
	y := 0
	prevY := -1 // the previous period's row in this section: the spine joins them
	last = "\x00"
	for _, p := range periods {
		if p.section != last {
			last = p.section
			prevY = -1
			if p.section != "" {
				cv.text(0, y, p.section, RoleClusterTitle, "", true)
				y++
			}
		}
		cv.text(nameW-width(p.name), y, p.name, RoleNodeText, "", true)
		cv.hline(nameW+1, spine+2, y, lineSolid, RoleEdge, "")
		if prevY >= 0 {
			cv.vline(spine, prevY, y, lineSolid, RoleEdge, "")
		}
		prevY = y
		ey := y
		for _, e := range p.events {
			for _, l := range wrap([]string{e}, evW) {
				cv.text(spine+4, ey, l, RoleNone, "", false)
				ey++
			}
		}
		y = max(y+1, ey) + 1
	}
	return cv.picture(), title, nil
}
