//! A grid being drawn on. Lines are recorded as directions per cell (up,
//! down, left, right) and turned into box-drawing characters at the end, so
//! crossings, junctions and corners join up whatever drew them.

use crate::asciitext::ascii_grapheme;
use crate::width::graphemes;
use crate::{Cell, Picture, Role};

#[derive(Clone, Copy, PartialEq, Eq, Debug, Default)]
pub(crate) enum LineStyle {
    #[default]
    Solid,
    Dotted,
    Thick,
    Invisible,
}

pub(crate) const BIT_UP: u8 = 1;
pub(crate) const BIT_DOWN: u8 = 2;
pub(crate) const BIT_LEFT: u8 = 4;
pub(crate) const BIT_RIGHT: u8 = 8;

#[derive(Clone, Default, Debug)]
pub(crate) struct CCell {
    pub bits: u8,
    pub style: LineStyle,
    pub mixed: bool,
    pub dbl: bool,
    pub round: bool,
    pub text: String,
    pub wide: bool,
    pub glyph: String,
    pub role: Role,
    pub color: String,
    pub bold: bool,
}

pub(crate) struct Canvas {
    pub w: i64,
    pub h: i64,
    pub cells: Vec<Vec<CCell>>,
    pub ascii: bool,
}

/// Which role a shared cell keeps: borders over lines.
fn role_priority(r: Role) -> i32 {
    match r {
        Role::Node | Role::Decision | Role::Actor | Role::Note => 3,
        Role::Cluster | Role::Block => 1,
        Role::Edge | Role::Lifeline => 2,
        _ => 0,
    }
}

impl Canvas {
    pub fn new(w: i64, h: i64, ascii: bool) -> Canvas {
        let (w, h) = (w.max(0), h.max(0));
        Canvas { w, h, cells: vec![vec![CCell::default(); w as usize]; h as usize], ascii }
    }

    pub fn at(&mut self, x: i64, y: i64) -> Option<&mut CCell> {
        if x < 0 || y < 0 || x >= self.w || y >= self.h {
            return None;
        }
        Some(&mut self.cells[y as usize][x as usize])
    }

    fn get(&self, x: i64, y: i64) -> Option<&CCell> {
        if x < 0 || y < 0 || x >= self.w || y >= self.h {
            return None;
        }
        Some(&self.cells[y as usize][x as usize])
    }

    pub fn add_bits(&mut self, x: i64, y: i64, bits: u8, st: LineStyle, role: Role, color: &str) {
        let Some(cell) = self.at(x, y) else { return };
        if cell.bits != 0 && cell.style != st {
            cell.mixed = true;
        }
        cell.bits |= bits;
        cell.style = st;
        if role_priority(role) >= role_priority(cell.role) {
            cell.role = role;
            if !color.is_empty() || role_priority(role) > 0 {
                cell.color = color.to_string();
            }
        }
    }

    /// A straight horizontal line between two cells (inclusive).
    pub fn hline(&mut self, x1: i64, x2: i64, y: i64, st: LineStyle, role: Role, color: &str) {
        let (x1, x2) = if x1 > x2 { (x2, x1) } else { (x1, x2) };
        for x in x1..=x2 {
            let mut b = 0;
            if x > x1 {
                b |= BIT_LEFT;
            }
            if x < x2 {
                b |= BIT_RIGHT;
            }
            if x1 == x2 {
                b = BIT_LEFT | BIT_RIGHT;
            }
            self.add_bits(x, y, b, st, role, color);
        }
    }

    pub fn vline(&mut self, x: i64, y1: i64, y2: i64, st: LineStyle, role: Role, color: &str) {
        let (y1, y2) = if y1 > y2 { (y2, y1) } else { (y1, y2) };
        for y in y1..=y2 {
            let mut b = 0;
            if y > y1 {
                b |= BIT_UP;
            }
            if y < y2 {
                b |= BIT_DOWN;
            }
            if y1 == y2 {
                b = BIT_UP | BIT_DOWN;
            }
            self.add_bits(x, y, b, st, role, color);
        }
    }

    /// A rectangle's border.
    #[allow(clippy::too_many_arguments)]
    pub fn draw_box(&mut self, x: i64, y: i64, w: i64, h: i64, st: LineStyle, round: bool, dbl: bool, role: Role, color: &str) {
        if w < 2 || h < 2 {
            return;
        }
        self.hline(x, x + w - 1, y, st, role, color);
        self.hline(x, x + w - 1, y + h - 1, st, role, color);
        self.vline(x, y, y + h - 1, st, role, color);
        self.vline(x + w - 1, y, y + h - 1, st, role, color);
        for (px, py) in [(x, y), (x + w - 1, y), (x, y + h - 1), (x + w - 1, y + h - 1)] {
            if let Some(c) = self.at(px, py) {
                c.round = round;
            }
        }
        if dbl {
            for yy in y..y + h {
                for xx in [x, x + w - 1] {
                    if let Some(c) = self.at(xx, yy) {
                        c.dbl = true;
                    }
                }
            }
            for xx in x..x + w {
                for yy in [y, y + h - 1] {
                    if let Some(c) = self.at(xx, yy) {
                        c.dbl = true;
                    }
                }
            }
        }
    }

    /// Writes s from (x, y); wide characters take two cells.
    pub fn text(&mut self, mut x: i64, y: i64, s: &str, role: Role, color: &str, bold: bool) {
        for (g, gw) in graphemes(s) {
            if self.get(x, y).is_none() {
                return;
            }
            if gw == 2 && self.get(x + 1, y).is_none() {
                return;
            }
            if self.ascii {
                // ASCII mode: every cell gets a plain character of its own.
                let a = ascii_grapheme(g, gw);
                {
                    let cell = self.at(x, y).unwrap();
                    cell.text = a[..1].to_string();
                    cell.role = role;
                    cell.color = color.to_string();
                    cell.bold = bold;
                    cell.wide = false;
                }
                if gw == 2 {
                    let n = self.at(x + 1, y).unwrap();
                    n.text = a[1..2].to_string();
                    n.wide = false;
                    n.role = role;
                    n.color = color.to_string();
                    n.bold = bold;
                }
                x += gw as i64;
                continue;
            }
            {
                let cell = self.at(x, y).unwrap();
                cell.text = g.to_string();
                cell.role = role;
                cell.color = color.to_string();
                cell.bold = bold;
                cell.wide = false;
            }
            if gw == 2 {
                let n = self.at(x + 1, y).unwrap();
                n.text = String::new();
                n.wide = true;
                n.role = role;
            }
            x += gw as i64;
        }
    }

    /// Puts a single symbol (an arrowhead, a corner mark) in a cell.
    pub fn glyph_at(&mut self, x: i64, y: i64, g: &str, role: Role, color: &str) {
        if let Some(c) = self.at(x, y) {
            c.glyph = g.to_string();
            c.role = role;
            c.color = color.to_string();
        }
    }

    /// Whether w cells from (x, y) are empty.
    pub fn blank(&self, x: i64, y: i64, w: i64) -> bool {
        for i in x..x + w {
            match self.get(i, y) {
                None => return false,
                Some(c) => {
                    if c.bits != 0 || !c.text.is_empty() || !c.glyph.is_empty() || c.wide {
                        return false;
                    }
                }
            }
        }
        true
    }

    fn resolve(&self, cell: &CCell) -> String {
        if cell.wide {
            return String::new();
        }
        if !cell.text.is_empty() {
            return cell.text.clone();
        }
        if !cell.glyph.is_empty() {
            if self.ascii {
                return ascii_grapheme(&cell.glyph, 1);
            }
            return cell.glyph.clone();
        }
        if cell.bits == 0 {
            return " ".to_string();
        }
        let b = cell.bits;
        let vert = b & (BIT_UP | BIT_DOWN) != 0;
        let horiz = b & (BIT_LEFT | BIT_RIGHT) != 0;
        if self.ascii {
            return if vert && horiz {
                "+"
            } else if vert {
                if cell.style == LineStyle::Dotted && !cell.mixed {
                    ":"
                } else {
                    "|"
                }
            } else if cell.style == LineStyle::Dotted && !cell.mixed {
                "."
            } else if cell.style == LineStyle::Thick && !cell.mixed {
                "="
            } else {
                "-"
            }
            .to_string();
        }
        if cell.dbl {
            if let Some(g) = double_glyph(b) {
                if (vert && horiz) || !cell.mixed {
                    return g.to_string();
                }
            }
        }
        if !cell.mixed {
            match cell.style {
                LineStyle::Thick => return heavy_glyph(b).to_string(),
                LineStyle::Dotted => {
                    if b == BIT_LEFT | BIT_RIGHT || b == BIT_LEFT || b == BIT_RIGHT {
                        return "┄".to_string();
                    }
                    if b == BIT_UP | BIT_DOWN || b == BIT_UP || b == BIT_DOWN {
                        return "┆".to_string();
                    }
                }
                _ => {}
            }
        }
        if cell.round {
            if let Some(g) = round_glyph(b) {
                return g.to_string();
            }
        }
        light_glyph(b).to_string()
    }

    /// Freezes the canvas, trimming blank rows and columns at the edges.
    pub fn picture(&self) -> Picture {
        let (mut min_x, mut min_y, mut max_x, mut max_y) = (self.w, self.h, -1i64, -1i64);
        for y in 0..self.h {
            for x in 0..self.w {
                let c = &self.cells[y as usize][x as usize];
                let s = self.resolve(c);
                if (s != " " && !s.is_empty()) || c.wide {
                    min_x = min_x.min(x);
                    max_x = max_x.max(x);
                    min_y = min_y.min(y);
                    max_y = max_y.max(y);
                }
            }
        }
        let mut p = Picture::default();
        if max_x < 0 {
            return p;
        }
        p.w = (max_x - min_x + 1) as usize;
        p.h = (max_y - min_y + 1) as usize;
        for y in 0..p.h {
            let mut row = Vec::with_capacity(p.w);
            for x in 0..p.w {
                let c = &self.cells[min_y as usize + y][min_x as usize + x];
                row.push(Cell { s: self.resolve(c), role: c.role, color: c.color.clone(), bold: c.bold, wide: c.wide });
            }
            p.cells.push(row);
        }
        p
    }
}

fn light_glyph(b: u8) -> &'static str {
    match b {
        x if x == BIT_LEFT | BIT_RIGHT || x == BIT_LEFT || x == BIT_RIGHT => "─",
        x if x == BIT_UP | BIT_DOWN || x == BIT_UP || x == BIT_DOWN => "│",
        x if x == BIT_DOWN | BIT_RIGHT => "┌",
        x if x == BIT_DOWN | BIT_LEFT => "┐",
        x if x == BIT_UP | BIT_RIGHT => "└",
        x if x == BIT_UP | BIT_LEFT => "┘",
        x if x == BIT_UP | BIT_DOWN | BIT_RIGHT => "├",
        x if x == BIT_UP | BIT_DOWN | BIT_LEFT => "┤",
        x if x == BIT_LEFT | BIT_RIGHT | BIT_DOWN => "┬",
        x if x == BIT_LEFT | BIT_RIGHT | BIT_UP => "┴",
        _ => "┼",
    }
}

fn heavy_glyph(b: u8) -> &'static str {
    match b {
        x if x == BIT_LEFT | BIT_RIGHT || x == BIT_LEFT || x == BIT_RIGHT => "━",
        x if x == BIT_UP | BIT_DOWN || x == BIT_UP || x == BIT_DOWN => "┃",
        x if x == BIT_DOWN | BIT_RIGHT => "┏",
        x if x == BIT_DOWN | BIT_LEFT => "┓",
        x if x == BIT_UP | BIT_RIGHT => "┗",
        x if x == BIT_UP | BIT_LEFT => "┛",
        x if x == BIT_UP | BIT_DOWN | BIT_RIGHT => "┣",
        x if x == BIT_UP | BIT_DOWN | BIT_LEFT => "┫",
        x if x == BIT_LEFT | BIT_RIGHT | BIT_DOWN => "┳",
        x if x == BIT_LEFT | BIT_RIGHT | BIT_UP => "┻",
        _ => "╋",
    }
}

fn double_glyph(b: u8) -> Option<&'static str> {
    Some(match b {
        x if x == BIT_LEFT | BIT_RIGHT || x == BIT_LEFT || x == BIT_RIGHT => "═",
        x if x == BIT_UP | BIT_DOWN || x == BIT_UP || x == BIT_DOWN => "║",
        x if x == BIT_DOWN | BIT_RIGHT => "╔",
        x if x == BIT_DOWN | BIT_LEFT => "╗",
        x if x == BIT_UP | BIT_RIGHT => "╚",
        x if x == BIT_UP | BIT_LEFT => "╝",
        x if x == BIT_UP | BIT_DOWN | BIT_RIGHT => "╠",
        x if x == BIT_UP | BIT_DOWN | BIT_LEFT => "╣",
        x if x == BIT_LEFT | BIT_RIGHT | BIT_DOWN => "╦",
        x if x == BIT_LEFT | BIT_RIGHT | BIT_UP => "╩",
        x if x == BIT_UP | BIT_DOWN | BIT_LEFT | BIT_RIGHT => "╬",
        _ => return None,
    })
}

fn round_glyph(b: u8) -> Option<&'static str> {
    Some(match b {
        x if x == BIT_DOWN | BIT_RIGHT => "╭",
        x if x == BIT_DOWN | BIT_LEFT => "╮",
        x if x == BIT_UP | BIT_RIGHT => "╰",
        x if x == BIT_UP | BIT_LEFT => "╯",
        _ => return None,
    })
}
