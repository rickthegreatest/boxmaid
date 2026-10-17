//! Drawing a laid-out level: frames, then edges, then nodes and labels on
//! top. Everything is computed in layout axes (o across, r along the flow)
//! and converted to screen coordinates here.

use crate::canvas::Canvas;
use crate::graph::{Graph, Head, LineStyle, Shape, ROOT};
use crate::layout::{Kind, Level};
use crate::text::width;
use crate::{Options, Picture, Role};

type Pt = (i64, i64);

/// Lays out and draws g.
fn layout_graph(g: &Graph, dir: &str, wrap: i64, ascii: bool) -> Picture {
    let mut root = Level::new(g, ROOT, dir, false, wrap, ascii);
    root.process();
    let mut cv = Canvas::new(root.w + 2, root.h + 2, ascii);
    root.draw(&mut cv, 0, 0);
    cv.picture()
}

/// Lays a graph out in the author's direction, wrapping labels tighter and
/// then trying the other direction until it fits the width.
pub(crate) fn fit_graph(g: &Graph, opt: &Options) -> Picture {
    let alt = match g.dir.as_str() {
        "LR" | "RL" => "TB",
        _ => "LR",
    };
    let mut best: Option<Picture> = None;
    for d in [g.dir.as_str(), alt] {
        for w in [28, 20, 14] {
            let p = layout_graph(g, d, w, opt.ascii);
            if opt.width == 0 || p.w <= opt.width {
                return p;
            }
            if best.as_ref().is_none_or(|b| p.w < b.w) {
                best = Some(p);
            }
        }
    }
    best.unwrap()
}

/// A multiplicity waiting to be written beside the end of a line.
pub(crate) struct EndMark {
    text: String,
    end: Pt,
    next: Pt,
    /// The line ends on the box's border (no arrowhead).
    on_border: bool,
}

impl Level<'_> {
    /// Converts a layout point to the level's screen coordinates.
    fn xy(&self, o: i64, r: i64) -> Pt {
        match self.dir.as_str() {
            "BT" => (o, self.rr - 1 - r),
            "LR" => (r, o),
            "RL" => (self.rr - 1 - r, o),
            _ => (o, r),
        }
    }

    /// An item's footprint on screen.
    fn rect(&self, i: usize) -> (i64, i64, i64, i64) {
        let it = &self.items[i];
        let (x1, y1) = self.xy(it.pos, it.r);
        let (x2, y2) = self.xy(it.pos + it.osize - 1, it.r + it.rsize - 1);
        (x1.min(x2), y1.min(y2), (x2 - x1).abs() + 1, (y2 - y1).abs() + 1)
    }

    pub(crate) fn draw(&mut self, cv: &mut Canvas, x0: i64, y0: i64) {
        if self.framed {
            cv.draw_box(x0, y0, self.w, self.h, LineStyle::Solid, true, false, Role::Cluster, "");
            let t = &self.g.cl[self.cl].title;
            if !t.is_empty() {
                cv.text(x0 + 2, y0, &format!(" {t} "), Role::ClusterTitle, "", true);
            }
        }
        for i in 0..self.items.len() {
            if self.items[i].kind == Kind::Cluster {
                let (x, y, _, _) = self.rect(i);
                self.items[i].sub.as_mut().unwrap().draw(cv, x0 + x, y0 + y);
            }
        }
        self.pending = vec![];
        for li in 0..self.edges.len() {
            if self.edge(&self.edges[li]).style != LineStyle::Invisible {
                self.draw_edge(cv, li, x0, y0);
            }
        }
        place_end_labels(cv, &self.pending);
        for i in 0..self.items.len() {
            match self.items[i].kind {
                Kind::Node => self.draw_node(cv, i, x0, y0),
                Kind::Label => {
                    let (x, y, w, h) = self.rect(i);
                    let lines = &self.items[i].lines;
                    for (k, line) in lines.iter().enumerate() {
                        let lw = width(line);
                        let ly = y + (h - lines.len() as i64) / 2 + k as i64;
                        cv.text(x0 + x + (w - lw) / 2, y0 + ly, line, Role::EdgeLabel, "", false);
                    }
                }
                _ => {}
            }
        }
    }

    /// The last segment leaving item it that belongs to level edge li.
    fn seg_of(&self, it: usize, li: usize) -> Option<usize> {
        self.items[it].downs.iter().copied().rfind(|&s| self.segs[s].le == li)
    }

    fn draw_edge(&mut self, cv: &mut Canvas, li: usize, x0: i64, y0: i64) {
        let le = self.edges[li].clone();
        let le = &le;
        let chain = &le.chain;
        if chain.len() < 2 {
            return;
        }
        let (a_head, b_head) = self.ends(le);
        let (stub_a, _, _) = self.stubs();
        let first = chain[0];
        let Some(s0) = self.seg_of(first, li) else { return };
        let fi = &self.items[first];
        let mut pts: Vec<Pt> = vec![];
        let mut start_r = fi.r + fi.rsize - 1;
        if fi.kind == Kind::Node && a_head != Head::None {
            start_r += 1;
        }
        pts.push((fi.pos + self.port_off(first, s0), start_r));
        let mut s = s0;
        loop {
            let (a, b) = (self.segs[s].a, self.segs[s].b);
            let xa = self.items[a].pos + self.port_off(a, s);
            let xb = self.items[b].pos + self.port_off(b, s);
            if xa != xb {
                let tr = self.gap_at.get(&self.items[a].rank).copied().unwrap_or(0) + stub_a + self.tracks.get(&s).copied().unwrap_or(0);
                pts.push((xa, tr));
                pts.push((xb, tr));
            }
            if b == le.b {
                let mut end_r = self.items[b].r;
                if self.items[b].kind == Kind::Node && b_head != Head::None {
                    end_r -= 1;
                }
                pts.push((xb, end_r));
                break;
            }
            let Some(next) = self.seg_of(b, li) else { return };
            s = next;
        }
        // To screen, then draw.
        let xs: Vec<Pt> = pts
            .iter()
            .map(|&(o, r)| {
                let (x, y) = self.xy(o, r);
                (x0 + x, y0 + y)
            })
            .collect();
        let e = self.edge(le);
        let st = e.style;
        for w in xs.windows(2) {
            let (p, q) = (w[0], w[1]);
            if p == q {
            } else if p.0 == q.0 {
                cv.vline(p.0, p.1, q.1, st, Role::Edge, "");
            } else if p.1 == q.1 {
                cv.hline(p.0, q.0, p.1, st, Role::Edge, "");
            }
        }
        if xs.len() == 1 {
            cv.vline(xs[0].0, xs[0].1, xs[0].1, st, Role::Edge, "");
        }
        if b_head != Head::None {
            let end = xs[xs.len() - 1];
            let mut prev = end;
            let mut i = xs.len() as i64 - 2;
            while i >= 0 && prev == end {
                prev = xs[i as usize];
                i -= 1;
            }
            cv.glyph_at(end.0, end.1, self.head_glyph(b_head, end, prev, true), Role::Arrow, "");
        }
        if !e.from_label.is_empty() || !e.to_label.is_empty() {
            let (from_end, to_end) = (xs[0], xs[xs.len() - 1]);
            let (from_next, to_prev) = (xs[1.min(xs.len() - 1)], xs[xs.len().saturating_sub(2)]);
            let (mut fl, mut tl) = (e.from_label.clone(), e.to_label.clone());
            if le.a_is_to {
                std::mem::swap(&mut fl, &mut tl);
            }
            self.pending.push(EndMark { text: fl, end: from_end, next: from_next, on_border: a_head == Head::None });
            self.pending.push(EndMark { text: tl, end: to_end, next: to_prev, on_border: b_head == Head::None });
        }
        if a_head != Head::None {
            let start = xs[0];
            let mut next = start;
            let mut i = 1;
            while i < xs.len() && next == start {
                next = xs[i];
                i += 1;
            }
            cv.glyph_at(start.0, start.1, self.head_glyph(a_head, start, next, false), Role::Arrow, "");
        }
    }

    /// Points an arrowhead from prev towards at. When the edge has no length
    /// (prev == at), it points along the flow.
    fn head_glyph(&self, h: Head, at: Pt, prev: Pt, forward: bool) -> &'static str {
        match h {
            Head::Circle => return "○",
            Head::Cross => return "×",
            Head::Diamond => return "◆",
            Head::DiamondOpen => return "◇",
            _ => {}
        }
        let (mut dx, mut dy) = (at.0 - prev.0, at.1 - prev.1);
        if dx == 0 && dy == 0 {
            // Along the flow, reversed for an arrow at the start of an edge.
            let (fx, fy) = self.xy(0, 1);
            let (ox, oy) = self.xy(0, 0);
            (dx, dy) = (fx - ox, fy - oy);
            if !forward {
                (dx, dy) = (-dx, -dy);
            }
        }
        let glyphs = if h == Head::Triangle { ["▽", "△", "▷", "◁"] } else { ["▼", "▲", "►", "◄"] };
        if dy > 0 {
            glyphs[0]
        } else if dy < 0 {
            glyphs[1]
        } else if dx > 0 {
            glyphs[2]
        } else {
            glyphs[3]
        }
    }

    fn draw_node(&self, cv: &mut Canvas, i: usize, x0: i64, y0: i64) {
        let it = &self.items[i];
        let n = &self.g.nodes[it.node.as_ref().unwrap()];
        let (fx, fy, _, _) = self.rect(i);
        let (x, y, w, h) = (x0 + fx + it.box_x, y0 + fy + it.box_y, it.box_w, it.box_h);
        let role = if n.shape == Shape::Rhombus { Role::Decision } else { Role::Node };
        let st = if n.dashed { LineStyle::Dotted } else { LineStyle::Solid };
        let col = n.color.as_str();
        match n.shape {
            Shape::Start => {
                cv.text(x, y, "●", Role::Start, col, false);
                return;
            }
            Shape::End => {
                cv.text(x, y, "◉", Role::Start, col, false);
                return;
            }
            Shape::Bar => {
                if self.lr() {
                    for k in 0..h {
                        cv.text(x, y + k, "┃", Role::Start, col, false);
                    }
                } else {
                    for k in 0..w {
                        cv.text(x + k, y, "━", Role::Start, col, false);
                    }
                }
                return;
            }
            Shape::Text => {}
            _ => {
                let round = matches!(n.shape, Shape::Round | Shape::Stadium | Shape::Circle | Shape::Cylinder);
                let dbl = matches!(n.shape, Shape::DoubleCircle | Shape::Subroutine);
                cv.draw_box(x, y, w, h, st, round, dbl, role, col);
                if n.shape == Shape::Subroutine {
                    for xx in x..x + w {
                        for yy in [y, y + h - 1] {
                            if let Some(c) = cv.at(xx, yy) {
                                c.dbl = false;
                            }
                        }
                    }
                }
                if n.shape == Shape::Cylinder {
                    cv.hline(x, x + w - 1, y + 1, st, role, col);
                }
                let corners: Option<[&str; 4]> = match n.shape {
                    Shape::Rhombus | Shape::Hexagon => Some(["╱", "╲", "╲", "╱"]),
                    Shape::LeanRight => Some(["╱", "╱", "╱", "╱"]),
                    Shape::LeanLeft => Some(["╲", "╲", "╲", "╲"]),
                    Shape::Trapezoid => Some(["╱", "╲", "", ""]),
                    Shape::TrapezoidAlt => Some(["", "", "╲", "╱"]),
                    _ => None,
                };
                if let Some(cs) = corners {
                    for (k, p) in [(x, y), (x + w - 1, y), (x, y + h - 1), (x + w - 1, y + h - 1)].iter().enumerate() {
                        if !cs[k].is_empty() {
                            cv.glyph_at(p.0, p.1, cs[k], role, col);
                        }
                    }
                }
            }
        }
        if !n.header.is_empty() || !n.sections.is_empty() {
            self.draw_compartments(cv, i, x, y, w, role, col);
            self.draw_loops(cv, i, x, y, w);
            return;
        }
        let lines = &it.lines;
        let mut total = lines.len() as i64;
        if !it.detail.is_empty() {
            total += 1 + it.detail.len() as i64;
        }
        let top = match n.shape {
            Shape::Cylinder => y + 1 + (h - 1 - total) / 2,
            Shape::Text => y,
            _ => y + (h - total) / 2,
        };
        for (k, line) in lines.iter().enumerate() {
            cv.text(x + (w - width(line)) / 2, top + k as i64, line, Role::NodeText, "", true);
        }
        for (k, line) in it.detail.iter().enumerate() {
            cv.text(x + (w - width(line)) / 2, top + lines.len() as i64 + 1 + k as i64, line, Role::Muted, "", false);
        }
        self.draw_loops(cv, i, x, y, w);
    }

    /// Writes a box made of compartments (a class, an entity, a
    /// requirement): header and name centred, then each section
    /// left-aligned below a dividing line.
    #[allow(clippy::too_many_arguments)]
    fn draw_compartments(&self, cv: &mut Canvas, i: usize, x: i64, y: i64, w: i64, role: Role, col: &str) {
        let it = &self.items[i];
        let n = &self.g.nodes[it.node.as_ref().unwrap()];
        let mut row = y + 1;
        for line in &n.header {
            cv.text(x + (w - width(line)) / 2, row, line, Role::Muted, "", false);
            row += 1;
        }
        for line in &it.lines {
            cv.text(x + (w - width(line)) / 2, row, line, Role::NodeText, "", true);
            row += 1;
        }
        for sec in &n.sections {
            cv.hline(x, x + w - 1, row, LineStyle::Solid, role, col);
            row += 1;
            for line in sec {
                cv.text(x + 2, row, line, Role::None, "", false);
                row += 1;
            }
            if sec.is_empty() {
                row += 1;
            }
        }
    }

    /// Draws edges from a node to itself beside it.
    fn draw_loops(&self, cv: &mut Canvas, i: usize, x: i64, y: i64, w: i64) {
        for (k, &ei) in self.items[i].loops.iter().enumerate() {
            let e = &self.g.edges[ei];
            if k > 0 || e.style == LineStyle::Invisible {
                break; // one loop is drawn; more would only overlap
            }
            let lbl = e.label.first().map_or("", |s| s.as_str());
            if self.lr() {
                let cx = x + w / 2;
                cv.vline(cx + 1, y - 2, y, e.style, Role::Edge, "");
                cv.hline(cx - 1, cx + 1, y - 2, e.style, Role::Edge, "");
                cv.vline(cx - 1, y - 2, y - 1, e.style, Role::Edge, "");
                cv.glyph_at(cx - 1, y - 1, "▼", Role::Arrow, "");
                if !lbl.is_empty() {
                    cv.text(cx + 3, y - 2, lbl, Role::EdgeLabel, "", false);
                }
                continue;
            }
            let (r1, r2) = (y + 1, y + 2);
            let right = x + w - 1;
            cv.hline(right, right + 2, r1, e.style, Role::Edge, "");
            cv.vline(right + 2, r1, r2, e.style, Role::Edge, "");
            cv.hline(right + 1, right + 2, r2, e.style, Role::Edge, "");
            cv.glyph_at(right + 1, r2, "◄", Role::Arrow, "");
            if !lbl.is_empty() {
                cv.text(right + 4, r1, lbl, Role::EdgeLabel, "", false);
            }
        }
    }
}

/// Writes multiplicities ("1", "0..*") once all lines are drawn: one step
/// out from the box, on whichever side of the line is free.
fn place_end_labels(cv: &mut Canvas, marks: &[EndMark]) {
    for m in marks {
        if m.text.is_empty() {
            continue;
        }
        let (mut x, mut y) = m.end;
        let (dx, dy) = ((m.next.0 - x).signum(), (m.next.1 - y).signum());
        if m.on_border {
            (x, y) = (x + dx, y + dy);
        }
        let w = width(&m.text);
        let spots: Vec<Pt> = if dx == 0 {
            // A vertical line: right of it, else left.
            vec![(x + 2, y), (x - 1 - w, y), (x + 2, y + dy), (x - 1 - w, y + dy)]
        } else {
            // Horizontal: above, else below.
            vec![(x, y - 1), (x, y + 1), (x - w + 1, y - 1)]
        };
        for p in spots {
            if cv.blank(p.0 - 1, p.1, w + 2) {
                cv.text(p.0, p.1, &m.text, Role::EdgeLabel, "", false);
                break;
            }
        }
    }
}
