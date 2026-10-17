package boxmaid

import (
	"regexp"
	"strconv"
	"strings"
)

// User journeys: each section, then its tasks with their score (1–5) as a
// bar coloured from bad to good, and who takes part.

var journeyTask = regexp.MustCompile(`^(.+?)\s*:\s*(\d+)\s*(?::\s*(.*))?$`)

func renderJourney(_, rest string, opt Options) (*Picture, string, error) {
	type task struct {
		name, section string
		score         int
		actors        string
	}
	title, section := "", ""
	var tasks []task
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
		if m := journeyTask.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			tasks = append(tasks, task{m[1], section, max(0, min(5, n)), strings.TrimSpace(m[3])})
		}
	}
	if len(tasks) == 0 {
		return nil, title, ErrEmpty
	}
	nameW := 6
	for _, t := range tasks {
		nameW = max(nameW, width(t.name))
	}
	if opt.Width > 0 {
		nameW = min(nameW, max(10, opt.Width-24))
	}
	rows := len(tasks)
	last := "\x00"
	for _, t := range tasks {
		if t.section != last {
			rows += 2
			last = t.section
		}
	}
	cv := newCanvas(nameW+80, rows+1, opt.ASCII)
	y := 0
	last = "\x00"
	for _, t := range tasks {
		if t.section != last {
			if last != "\x00" {
				y++
			}
			last = t.section
			if t.section != "" {
				cv.text(0, y, t.section, RoleClusterTitle, "", true)
				y++
			}
		}
		cv.text(2, y, truncate(t.name, nameW), RoleNodeText, "", false)
		role := RoleGood
		switch {
		case t.score <= 2:
			role = RoleWarn
		case t.score == 3:
			role = RoleAccent
		}
		x := nameW + 4
		filled, empty := "●", "○"
		if opt.ASCII {
			filled, empty = "#", "."
		}
		cv.text(x, y, strings.Repeat(filled, t.score), role, "", false)
		cv.text(x+t.score, y, strings.Repeat(empty, 5-t.score), RoleMuted, "", false)
		cv.text(x+6, y, strconv.Itoa(t.score), role, "", true)
		if t.actors != "" {
			cv.text(x+9, y, t.actors, RoleMuted, "", false)
		}
		y++
	}
	return cv.picture(), title, nil
}
