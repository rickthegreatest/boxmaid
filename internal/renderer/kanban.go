package renderer

import (
	"regexp"
	"strconv"
	"strings"
)

// Kanban boards and packet diagrams.

var (
	kanbanItem = regexp.MustCompile(`^([\w-]+)?\s*(?:\[(.*?)\])?\s*(?:@\{(.*)\})?\s*$`)
	kanbanMeta = regexp.MustCompile(`(\w+)\s*:\s*('[^']*'|"[^"]*"|[^,}]+)`)
	packetRow  = regexp.MustCompile(`^(?:(\d+)(?:\s*-\s*(\d+))?|\+(\d+))\s*:\s*"([^"]*)"`)
)

// renderKanban draws the columns side by side, each with its cards (and
// their ticket, assignee and priority); columns that don't fit the width
// wrap to a new row of columns.
func renderKanban(_, rest string, opt Options) (*Picture, string, error) {
	type card struct {
		text []string
		meta string
		prio string
	}
	type column struct {
		title string
		cards []card
	}
	var cols []*column
	colIndent := -1
	for _, raw := range strings.Split(rest, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		m := kanbanItem.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			continue
		}
		text := m[2]
		if text == "" {
			text = m[1]
		}
		if colIndent < 0 || indent <= colIndent {
			colIndent = indent
			cols = append(cols, &column{title: text})
			continue
		}
		c := card{text: label(text)}
		var meta []string
		for _, kv := range kanbanMeta.FindAllStringSubmatch(m[3], -1) {
			v := strings.Trim(strings.TrimSpace(kv[2]), `'"`)
			switch kv[1] {
			case "priority":
				c.prio = v
			case "assigned":
				meta = append(meta, "@"+v)
			default:
				meta = append(meta, v)
			}
		}
		c.meta = strings.Join(meta, " · ")
		cols[len(cols)-1].cards = append(cols[len(cols)-1].cards, c)
	}
	if len(cols) == 0 {
		return nil, "", ErrEmpty
	}
	W := opt.Width
	if W <= 0 {
		W = 120
	}
	colW := 26
	perRow := max(1, (W+1)/(colW+1))
	if len(cols) < perRow {
		colW = min(36, max(colW, (W+1)/len(cols)-1))
		perRow = len(cols)
	}
	inner := colW - 6
	// Height of each band of columns.
	cardH := func(c card) int {
		h := len(wrap(c.text, inner)) + 2
		if c.meta != "" || c.prio != "" {
			h++
		}
		return h
	}
	var bands [][]*column
	for i := 0; i < len(cols); i += perRow {
		bands = append(bands, cols[i:min(len(cols), i+perRow)])
	}
	total := 0
	bandH := make([]int, len(bands))
	for bi, band := range bands {
		for _, c := range band {
			h := 2
			for _, cd := range c.cards {
				h += cardH(cd)
			}
			bandH[bi] = max(bandH[bi], h+1)
		}
		total += bandH[bi] + 1
	}
	cv := newCanvas(perRow*(colW+1)+1, total+1, opt.ASCII)
	y0 := 0
	for bi, band := range bands {
		for ci, c := range band {
			x := ci * (colW + 1)
			cv.box(x, y0, colW, bandH[bi], lineSolid, true, false, RoleCluster, "")
			cv.text(x+2, y0, " "+truncate(c.title, colW-6)+" ", RoleClusterTitle, "", true)
			cv.text(x+colW-2-width(strconv.Itoa(len(c.cards))), y0, strconv.Itoa(len(c.cards)), RoleMuted, "", false)
			y := y0 + 1
			for _, cd := range c.cards {
				h := cardH(cd)
				cv.box(x+1, y, colW-2, h, lineSolid, false, false, RoleNode, "")
				for i, l := range wrap(cd.text, inner) {
					cv.text(x+3, y+1+i, l, RoleNodeText, "", false)
				}
				if cd.meta != "" || cd.prio != "" {
					line := cd.meta
					role := RoleMuted
					if cd.prio != "" {
						cv.text(x+colW-4-width(cd.prio), y+h-2, cd.prio, priorityRole(cd.prio), "", true)
					}
					cv.text(x+3, y+h-2, truncate(line, inner-width(cd.prio)-1), role, "", false)
				}
				y += h
			}
		}
		y0 += bandH[bi] + 1
	}
	return cv.picture(), "", nil
}

func priorityRole(p string) Role {
	switch strings.ToLower(p) {
	case "very high", "high":
		return RoleWarn
	case "low", "very low":
		return RoleGood
	}
	return RoleAccent
}

// renderPacket draws a packet's fields as boxes on rows of 32 bits, with
// the bit numbers above each row.
func renderPacket(_, rest string, opt Options) (*Picture, string, error) {
	type field struct {
		lo, hi int
		name   string
	}
	var fields []field
	title := ""
	next := 0
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "title ") {
			title = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "title")), `"`)
			continue
		}
		m := packetRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var lo, hi int
		if m[3] != "" { // +n: the next n bits
			n, _ := strconv.Atoi(m[3])
			lo, hi = next, next+max(1, n)-1
		} else {
			lo, _ = strconv.Atoi(m[1])
			hi = lo
			if m[2] != "" {
				hi, _ = strconv.Atoi(m[2])
			}
		}
		fields = append(fields, field{lo, hi, m[4]})
		next = hi + 1
	}
	if len(fields) == 0 {
		return nil, title, ErrEmpty
	}
	const bitsPerRow = 32
	cellW := 3
	if opt.Width > 0 {
		cellW = max(1, min(3, (opt.Width-1)/bitsPerRow))
	}
	rows := (next + bitsPerRow - 1) / bitsPerRow
	cv := newCanvas(bitsPerRow*cellW+2, rows*4+1, opt.ASCII)
	for r := 0; r < rows; r++ {
		y := r * 4
		for _, f := range fields {
			lo, hi := max(f.lo, r*bitsPerRow), min(f.hi, r*bitsPerRow+bitsPerRow-1)
			if lo > hi {
				continue
			}
			x1 := (lo - r*bitsPerRow) * cellW
			x2 := (hi - r*bitsPerRow + 1) * cellW
			cv.box(x1, y+1, x2-x1+1, 3, lineSolid, false, false, RoleNode, "")
			name := truncate(f.name, x2-x1-1)
			cv.text(x1+(x2-x1+1-width(name))/2, y+2, name, RoleNodeText, "", false)
			// Bit numbers just inside the field's edges, so neighbours'
			// numbers never run together.
			cv.text(x1+1, y, strconv.Itoa(lo), RoleMuted, "", false)
			if end := strconv.Itoa(hi); hi != lo && x2-width(end) > x1+1+width(strconv.Itoa(lo)) {
				cv.text(x2-width(end), y, end, RoleMuted, "", false)
			}
		}
	}
	return cv.picture(), title, nil
}
