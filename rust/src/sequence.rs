//! Sequence diagrams: participants side by side with lifelines down from
//! them, messages as arrows between lifelines, top to bottom in order.

use std::collections::HashMap;

use crate::canvas::{CCell, Canvas, LineStyle, BIT_DOWN, BIT_UP};
use crate::graph::Head;
use crate::re::{group, lazy_re};
use crate::text::{label, max_width, width, wrap};
use crate::{Drawn, Error, Options, Picture, Role};

struct Participant {
    label: Vec<String>,
    actor: bool,
    /// Lifeline column.
    x: i64,
    /// Box width.
    w: i64,
    group: Option<usize>,
    /// Row where a created participant appears, or -1.
    created: i64,
    idx: usize,
}

struct Group {
    title: String,
    first: i64,
    last: i64,
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Kind {
    Msg,
    Note,
    Block,
    Else,
    End,
    Activate,
    Deactivate,
    Create,
}

struct Event {
    kind: Kind,
    from: usize,
    to: usize,
    label: Vec<String>,
    line: LineStyle,
    head: Head,
    tail: Head,
    /// The message activates its target / deactivates its source.
    act: bool,
    deact: bool,
    /// Notes: left, right, over.
    side: String,
    block: usize,
    /// activate/deactivate/create
    who: usize,
}

impl Event {
    fn new(kind: Kind) -> Event {
        Event {
            kind,
            from: 0,
            to: 0,
            label: vec![],
            line: LineStyle::Solid,
            head: Head::None,
            tail: Head::None,
            act: false,
            deact: false,
            side: String::new(),
            block: 0,
            who: 0,
        }
    }
}

struct Block {
    kind: String,
    label: String,
    parent: Option<usize>,
    /// Participants it spans.
    lo: i64,
    hi: i64,
    /// How many blocks nest inside it.
    depth: i64,
    top: i64,
    bottom: i64,
    elses: Vec<Else>,
}

struct Else {
    row: i64,
    label: String,
    kind: String,
}

lazy_re!(SEQ_MSG, r"^(.+?)\s*(<<-->>|<<->>|-->>|->>|-->|->|--x|-x|--\)|-\))\s*([+-]?)\s*([^:]+?)\s*(?::\s?(.*))?$");
lazy_re!(SEQ_PART, r"^(participant|actor)\s+(.+?)(?:\s+as\s+(.+))?$");
lazy_re!(SEQ_NOTE, r"(?i)^note\s+(left of|right of|over)\s+([^:]+?)\s*:\s*(.*)$");

#[derive(Default)]
struct Parser {
    parts: Vec<Participant>,
    by_id: HashMap<String, usize>,
    events: Vec<Event>,
    blocks: Vec<Block>,
    stack: Vec<usize>,
    groups: Vec<Group>,
    group: Option<usize>,
    auto: bool,
    n: i64,
    title: String,
}

impl Parser {
    fn part(&mut self, id: &str) -> usize {
        let id = id.trim();
        if let Some(&q) = self.by_id.get(id) {
            return q;
        }
        let idx = self.parts.len();
        if let Some(g) = self.group {
            self.groups[g].last = idx as i64;
        }
        self.parts.push(Participant { label: label(id), actor: false, x: 0, w: 0, group: self.group, created: -1, idx });
        self.by_id.insert(id.to_string(), idx);
        idx
    }

    /// Widens the open blocks to cover participants a..b.
    fn touch(&mut self, a: usize, b: usize) {
        let (a, b) = (a.min(b) as i64, a.max(b) as i64);
        for &bl in &self.stack {
            let bl = &mut self.blocks[bl];
            if bl.lo < 0 || a < bl.lo {
                bl.lo = a;
            }
            bl.hi = bl.hi.max(b);
        }
    }
}

fn parse_sequence(src: &str) -> Result<Parser, Error> {
    let mut p = Parser::default();
    for raw in src.split('\n') {
        let t = raw.trim();
        let line = t.strip_suffix(';').unwrap_or(t).trim();
        if line.is_empty() {
            continue;
        }
        let (word, rest) = line.split_once(' ').unwrap_or((line, ""));
        let rest = rest.trim();
        match word.to_lowercase().as_str() {
            "title" => {
                p.title = rest.strip_prefix(':').unwrap_or(rest).trim().to_string();
                continue;
            }
            "title:" => {
                p.title = rest.to_string();
                continue;
            }
            "autonumber" => {
                p.auto = true;
                continue;
            }
            "link" | "links" | "acctitle" | "accdescr" | "acctitle:" | "accdescr:" | "destroy" => continue,
            "box" => {
                let n = p.parts.len() as i64;
                p.groups.push(Group { title: box_title(rest), first: n, last: n - 1 });
                p.group = Some(p.groups.len() - 1);
                continue;
            }
            "create" => {
                if let Some(m) = SEQ_PART.captures(rest) {
                    let q = p.part(&m[2]);
                    p.parts[q].actor = &m[1] == "actor";
                    if !group(&m, 3).is_empty() {
                        p.parts[q].label = label(group(&m, 3));
                    }
                    let mut e = Event::new(Kind::Create);
                    e.who = q;
                    p.events.push(e);
                }
                continue;
            }
            "activate" | "deactivate" => {
                let q = p.part(rest);
                let mut e = Event::new(if word == "deactivate" { Kind::Deactivate } else { Kind::Activate });
                e.who = q;
                p.events.push(e);
                continue;
            }
            "loop" | "alt" | "opt" | "par" | "critical" | "break" | "rect" => {
                let lbl = if word == "rect" { "" } else { rest };
                let bl = Block {
                    kind: word.to_string(),
                    label: label(lbl).join(" "),
                    parent: p.stack.last().copied(),
                    lo: -1,
                    hi: -1,
                    depth: 0,
                    top: 0,
                    bottom: 0,
                    elses: vec![],
                };
                p.blocks.push(bl);
                let i = p.blocks.len() - 1;
                p.stack.push(i);
                let mut e = Event::new(Kind::Block);
                e.block = i;
                p.events.push(e);
                continue;
            }
            "else" | "and" | "option" => {
                if let Some(&b) = p.stack.last() {
                    let mut e = Event::new(Kind::Else);
                    e.block = b;
                    e.label = vec![word.to_string(), label(rest).join(" ")];
                    p.events.push(e);
                }
                continue;
            }
            "end" => {
                if let Some(b) = p.stack.pop() {
                    let mut e = Event::new(Kind::End);
                    e.block = b;
                    p.events.push(e);
                } else if p.group.is_some() {
                    p.group = None;
                }
                continue;
            }
            _ => {}
        }
        if let Some(m) = SEQ_PART.captures(line) {
            let q = p.part(&m[2]);
            p.parts[q].actor = &m[1] == "actor";
            if !group(&m, 3).is_empty() {
                p.parts[q].label = label(group(&m, 3));
            }
            continue;
        }
        if let Some(m) = SEQ_NOTE.captures(line) {
            let ids: Vec<&str> = m[2].split(',').collect();
            let mut a = p.part(ids[0]);
            let mut b = a;
            if ids.len() > 1 {
                b = p.part(ids[1]);
            }
            if a > b {
                std::mem::swap(&mut a, &mut b);
            }
            let side = m[1].split_whitespace().next().unwrap_or("").to_lowercase();
            let (lo, mut hi) = (a, b);
            if side == "right" && hi + 1 < p.parts.len() {
                hi += 1;
            }
            let mut e = Event::new(Kind::Note);
            (e.from, e.to, e.side, e.label) = (a, b, side, label(&m[3]));
            p.events.push(e);
            p.touch(lo, hi);
            continue;
        }
        if let Some(m) = SEQ_MSG.captures(line) {
            let (from, to) = (p.part(&m[1]), p.part(&m[4]));
            let mut e = Event::new(Kind::Msg);
            (e.from, e.to, e.label) = (from, to, label(group(&m, 5)));
            if e.label.len() == 1 && e.label[0].is_empty() {
                e.label = vec![];
            }
            let op = &m[2];
            if op.trim_matches('<').contains("--") {
                e.line = LineStyle::Dotted;
            }
            if op.ends_with(">>") || op.ends_with(')') {
                e.head = Head::Arrow;
            } else if op.ends_with('x') {
                e.head = Head::Cross;
            }
            if op.starts_with("<<") {
                e.tail = Head::Arrow;
            }
            (e.act, e.deact) = (&m[3] == "+", &m[3] == "-");
            if p.auto {
                p.n += 1;
                let first = e.label.first().cloned().unwrap_or_default();
                let mut l = vec![format!("{} {}", circled(p.n), first)];
                if e.label.len() >= 2 {
                    l.extend_from_slice(&e.label[1..]);
                }
                e.label = l;
            }
            p.events.push(e);
            let mut hi = from.max(to);
            if from == to && hi + 1 < p.parts.len() {
                hi += 1; // a message to itself loops out to the right
            }
            p.touch(from.min(to), hi);
            continue;
        }
    }
    if p.parts.is_empty() {
        return Err(Error::Empty);
    }
    Ok(p)
}

fn circled(n: i64) -> String {
    if (1..=20).contains(&n) {
        return char::from_u32('①' as u32 + n as u32 - 1).unwrap().to_string();
    }
    format!("({n})")
}

/// Drops a box's colour ("box Aqua Payments", "box rgb(…) Payments").
fn box_title(s: &str) -> String {
    let mut s = s.trim().to_string();
    if s.starts_with("rgb") || s.starts_with('#') {
        if let Some(i) = s.find([')', ' ']) {
            s = s[i + 1..].trim().to_string();
        }
    } else {
        let f: Vec<&str> = s.split_whitespace().collect();
        if f.len() > 1 && is_color_name(f[0]) {
            s = f[1..].join(" ");
        }
    }
    label(&s).join(" ")
}

fn is_color_name(s: &str) -> bool {
    matches!(
        s.to_lowercase().as_str(),
        "aqua"
            | "red"
            | "green"
            | "blue"
            | "yellow"
            | "orange"
            | "purple"
            | "pink"
            | "grey"
            | "gray"
            | "white"
            | "black"
            | "lightblue"
            | "lightgreen"
            | "lightyellow"
            | "lightgrey"
            | "lightgray"
            | "transparent"
            | "teal"
            | "olive"
            | "silver"
            | "beige"
    )
}

/// Parses and draws a sequence diagram, wrapping message labels tighter
/// until it fits.
pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut p = parse_sequence(rest)?;
    let mut best: Option<Picture> = None;
    for wrap_at in [40, 28, 20, 14] {
        let pic = p.draw(wrap_at, opt.ascii);
        if opt.width == 0 || pic.w <= opt.width {
            return Ok((pic, p.title));
        }
        if best.as_ref().is_none_or(|b| pic.w < b.w) {
            best = Some(pic);
        }
    }
    Ok((best.unwrap(), p.title))
}

impl Parser {
    // Like the Go original, drawing keeps what it learns (block depths,
    // else rows) on the parser between attempts.
    fn draw(&mut self, wrap_at: i64, ascii: bool) -> Picture {
        let n = self.parts.len();
        for q in &mut self.parts {
            q.w = max_width(&wrap(&q.label, 20)) + 4;
        }
        let labels: Vec<Vec<String>> =
            self.events.iter().map(|e| if e.kind == Kind::Msg || e.kind == Kind::Note { wrap(&e.label, wrap_at) } else { vec![] }).collect();
        let parts = &mut self.parts;
        // Gaps between neighbouring lifelines.
        let mut gap: Vec<i64> = (0..n.saturating_sub(1))
            .map(|i| {
                let mut g = (parts[i].w + parts[i + 1].w) / 2 + 3;
                if parts[i].group != parts[i + 1].group {
                    g += 2;
                }
                g
            })
            .collect();
        let mut right = parts[n - 1].w / 2 + 2; // room right of the last lifeline
        let mut left = parts[0].w / 2 + 2;
        // Lifelines a..b (a<b) must be w apart.
        let need = |gap: &mut Vec<i64>, a: usize, b: usize, w: i64| {
            let sum: i64 = gap[a..b].iter().sum();
            if sum < w {
                gap[b - 1] += w - sum;
            }
        };
        for (ei, e) in self.events.iter().enumerate() {
            let lw = max_width(&labels[ei]);
            match e.kind {
                Kind::Msg => {
                    let (a, b) = (e.from.min(e.to), e.from.max(e.to));
                    if a == b {
                        if a < n - 1 {
                            need(&mut gap, a, a + 1, lw + 8);
                        } else {
                            right = right.max(lw + 8);
                        }
                        continue;
                    }
                    need(&mut gap, a, b, lw + 6);
                }
                Kind::Note => {
                    let nw = lw + 4;
                    match e.side.as_str() {
                        "right" => {
                            if e.from < n - 1 {
                                need(&mut gap, e.from, e.from + 1, nw + 3);
                            } else {
                                right = right.max(nw + 3);
                            }
                        }
                        "left" => {
                            if e.from > 0 {
                                need(&mut gap, e.from - 1, e.from, nw + 3);
                            } else {
                                left = left.max(nw + 3);
                            }
                        }
                        _ => {
                            if e.from == e.to {
                                if e.from == 0 {
                                    left = left.max(nw / 2 + 1);
                                }
                                if e.from == n - 1 {
                                    right = right.max(nw / 2 + 1);
                                }
                            } else {
                                need(&mut gap, e.from, e.to, nw - 4);
                            }
                        }
                    }
                }
                _ => {}
            }
        }
        for i in 0..self.blocks.len() {
            if self.blocks[i].lo < 0 {
                self.blocks[i].lo = 0;
                self.blocks[i].hi = n as i64 - 1;
            }
            let mut d = 0;
            let mut c = i;
            while let Some(par) = self.blocks[c].parent {
                self.blocks[par].depth = self.blocks[par].depth.max(d + 1);
                d += 1;
                c = par;
            }
            let bl = &self.blocks[i];
            let lw = width(&bl.kind) + width(&bl.label) + 8;
            if bl.lo < bl.hi {
                need(&mut gap, bl.lo as usize, bl.hi as usize, lw - 6);
            }
        }
        let max_depth = self.blocks.iter().map(|b| b.depth + 1).max().unwrap_or(0);
        left += 3 + 2 * max_depth;
        right += 3 + 2 * max_depth;
        let mut x = left;
        for (i, q) in parts.iter_mut().enumerate() {
            q.x = x;
            if i < n - 1 {
                x += gap[i];
            }
        }
        let w = x + right;
        // Rows.
        let box_h = parts.iter().map(|q| wrap(&q.label, 20).len() as i64 + 2).fold(3, i64::max);
        let has_groups = parts.iter().any(|q| q.group.is_some());
        let top = if has_groups { 2 } else { 0 };
        let mut y = top + box_h + 1;
        let mut rows: Vec<i64> = vec![0; self.events.len()];
        let mut active_from: Vec<Vec<i64>> = vec![vec![]; n];
        let mut spans: Vec<(usize, i64, i64)> = vec![];
        let deactivate = |active_from: &mut Vec<Vec<i64>>, spans: &mut Vec<(usize, i64, i64)>, who: usize, row: i64| {
            if let Some(f) = active_from[who].pop() {
                spans.push((who, f, row));
            }
        };
        for (ei, e) in self.events.iter().enumerate() {
            match e.kind {
                Kind::Create => {
                    parts[e.who].created = y;
                    y += box_h;
                }
                Kind::Msg => {
                    let lines = &labels[ei];
                    if e.from == e.to {
                        rows[ei] = y;
                        y += (lines.len() as i64 + 1).max(2) + 1;
                        continue;
                    }
                    y += lines.len() as i64;
                    rows[ei] = y;
                    if e.act {
                        active_from[e.to].push(y);
                    }
                    if e.deact {
                        deactivate(&mut active_from, &mut spans, e.from, y);
                    }
                    y += 2;
                }
                Kind::Note => {
                    rows[ei] = y;
                    y += labels[ei].len() as i64 + 3;
                }
                Kind::Block => {
                    self.blocks[e.block].top = y;
                    self.blocks[e.block].elses.clear(); // from an earlier, wider attempt
                    y += 2;
                }
                Kind::Else => {
                    self.blocks[e.block].elses.push(Else { row: y, kind: e.label[0].clone(), label: e.label[1].clone() });
                    y += 2;
                }
                Kind::End => {
                    self.blocks[e.block].bottom = y;
                    y += 2;
                }
                Kind::Activate => active_from[e.who].push(y),
                Kind::Deactivate => deactivate(&mut active_from, &mut spans, e.who, y),
            }
        }
        for who in 0..n {
            while !active_from[who].is_empty() {
                deactivate(&mut active_from, &mut spans, who, y);
            }
        }
        let bottom_y = y;
        let mut h = bottom_y + box_h + 1;
        if has_groups {
            h += 1;
        }
        let mut cv = Canvas::new(w + 2, h + 2, ascii);
        // Participant groups.
        for q in parts.iter() {
            let Some(gi) = q.group else { continue };
            let g = &self.groups[gi];
            if q.idx as i64 == g.first {
                let (l, r) = (&parts[g.first as usize], &parts[g.first.max(g.last) as usize]);
                let (x1, x2) = (l.x - l.w / 2 - 2, r.x + r.w / 2 + 2);
                cv.draw_box(x1, 0, x2 - x1 + 1, h, LineStyle::Dotted, true, false, Role::Cluster, "");
                if !g.title.is_empty() {
                    cv.text(x1 + 2, 0, &format!(" {} ", g.title), Role::ClusterTitle, "", true);
                }
            }
        }
        // Blocks.
        for bl in self.blocks.iter_mut() {
            let (l, r) = (&parts[bl.lo as usize], &parts[bl.hi as usize]);
            let m = 3 + 2 * bl.depth;
            let x1 = l.x - m;
            let mut x2 = r.x + m;
            if bl.hi == n as i64 - 1 {
                x2 = x2.max(w - 2 - 2 * (max_depth - 1 - bl.depth));
            }
            if bl.top == 0 && bl.bottom == 0 {
                continue;
            }
            if bl.bottom <= bl.top {
                bl.bottom = bottom_y - 1;
            }
            cv.draw_box(x1, bl.top, x2 - x1 + 1, bl.bottom - bl.top + 1, LineStyle::Solid, false, false, Role::Block, "");
            if bl.kind != "rect" {
                cv.text(x1 + 1, bl.top, &format!(" {} ", bl.kind), Role::BlockTitle, "", true);
                if !bl.label.is_empty() {
                    cv.text(x1 + width(&bl.kind) + 3, bl.top, &format!("[{}] ", bl.label), Role::BlockTitle, "", false);
                }
            }
            for el in &bl.elses {
                cv.hline(x1, x2, el.row, LineStyle::Dotted, Role::Block, "");
                cv.add_bits(x1, el.row, BIT_UP | BIT_DOWN, LineStyle::Solid, Role::Block, "");
                cv.add_bits(x2, el.row, BIT_UP | BIT_DOWN, LineStyle::Solid, Role::Block, "");
                let mut t = format!(" {} ", el.kind);
                if !el.label.is_empty() {
                    t += &format!("[{}] ", el.label);
                }
                cv.text(x1 + 1, el.row, &t, Role::BlockTitle, "", false);
            }
        }
        // Lifelines, thick where active.
        for q in parts.iter() {
            let from = if q.created >= 0 { q.created + box_h } else { top + box_h };
            cv.vline(q.x, from, bottom_y, LineStyle::Dotted, Role::Lifeline, "");
        }
        for &(who, from, to) in &spans {
            for r in from..=to {
                if let Some(c) = cv.at(parts[who].x, r) {
                    c.style = LineStyle::Thick;
                    c.mixed = false;
                }
            }
        }
        // Messages.
        for (ei, e) in self.events.iter().enumerate() {
            match e.kind {
                Kind::Msg => {
                    let lines = &labels[ei];
                    let r = rows[ei];
                    let (from, to) = (&parts[e.from], &parts[e.to]);
                    if e.from == e.to {
                        let x = from.x;
                        cv.hline(x, x + 3, r, e.line, Role::Edge, "");
                        cv.vline(x + 3, r, r + 1, e.line, Role::Edge, "");
                        cv.hline(x + 1, x + 3, r + 1, e.line, Role::Edge, "");
                        if e.head != Head::None {
                            cv.glyph_at(x + 1, r + 1, head_glyph_for(e.head, "◄"), Role::Arrow, "");
                        }
                        for (i, line) in lines.iter().enumerate() {
                            cv.text(x + 5, r + i as i64, line, Role::EdgeLabel, "", false);
                        }
                        continue;
                    }
                    let dir = if to.x < from.x { -1 } else { 1 };
                    cv.hline(from.x, to.x - dir, r, e.line, Role::Edge, "");
                    if e.head != Head::None {
                        let g = if dir < 0 { "◄" } else { "►" };
                        cv.glyph_at(to.x - dir, r, head_glyph_for(e.head, g), Role::Arrow, "");
                    }
                    if e.tail != Head::None {
                        let g = if dir < 0 { "►" } else { "◄" };
                        cv.glyph_at(from.x + dir, r, g, Role::Arrow, "");
                    }
                    let mid = (from.x + to.x) / 2;
                    for (i, line) in lines.iter().enumerate() {
                        cv.text(mid - width(line) / 2, r - lines.len() as i64 + i as i64, line, Role::EdgeLabel, "", false);
                    }
                }
                Kind::Note => {
                    let lines = &labels[ei];
                    let mut nw = max_width(lines) + 4;
                    let (a, b) = (&parts[e.from], &parts[e.to]);
                    let x1 = match e.side.as_str() {
                        "right" => a.x + 2,
                        "left" => a.x - 2 - nw + 1,
                        _ => {
                            if e.from == e.to {
                                a.x - nw / 2
                            } else {
                                nw = nw.max(b.x - a.x + 5);
                                a.x - 2
                            }
                        }
                    };
                    let r = rows[ei];
                    let lh = lines.len() as i64;
                    // A note covers the lifelines behind it.
                    for yy in r..r + lh + 2 {
                        for xx in x1..x1 + nw {
                            if let Some(c) = cv.at(xx, yy) {
                                *c = CCell::default();
                            }
                        }
                    }
                    cv.draw_box(x1, r, nw, lh + 2, LineStyle::Solid, false, false, Role::Note, "");
                    for (i, line) in lines.iter().enumerate() {
                        cv.text(x1 + 2, r + 1 + i as i64, line, Role::Note, "", false);
                    }
                }
                _ => {}
            }
        }
        // Participant boxes, top and bottom.
        for q in parts.iter() {
            let lines = wrap(&q.label, 20);
            let first = if q.created >= 0 { q.created } else { top };
            for by in [first, bottom_y + 1] {
                let bx = q.x - q.w / 2;
                cv.draw_box(bx, by, q.w, box_h, LineStyle::Solid, q.actor, false, Role::Actor, "");
                for (i, line) in lines.iter().enumerate() {
                    let ly = by + 1 + (box_h - 2 - lines.len() as i64) / 2 + i as i64;
                    cv.text(q.x - width(line) / 2, ly, line, Role::NodeText, "", true);
                }
            }
        }
        cv.picture()
    }
}

fn head_glyph_for(h: Head, arrow: &str) -> &str {
    match h {
        Head::Cross => "×",
        Head::Circle => "○",
        _ => arrow,
    }
}
