//! Quadrant charts and XY charts.

use std::collections::HashMap;

use crate::canvas::{Canvas, LineStyle};
use crate::entities::format_number;
use crate::gitgraph::GIT_COLORS;
use crate::re::{group, lazy_re};
use crate::text::{max_width, split_top_level, width};
use crate::width::truncate;
use crate::{Drawn, Error, Options, Picture, Role};

lazy_re!(QUAD_POINT, r"^(.+?)\s*:\s*\[\s*([\d.]+)\s*,\s*([\d.]+)\s*\]");
lazy_re!(AXIS_RANGE, r#"^(?:"([^"]*)"|(\S+(?:\s+\S+)*?))?\s*(-?[\d.]+)\s*-->\s*(-?[\d.]+)\s*$"#);
lazy_re!(AXIS_LIST, r#"^(?:"([^"]*)"|([^\[]*?))?\s*\[(.*)\]\s*$"#);
lazy_re!(SERIES_RE, r#"^(bar|line)\s*(?:"[^"]*"\s*)?\[(.*)\]\s*$"#);

/// Like Go's strconv.ParseFloat: the number, or an error when it isn't one.
pub(crate) fn parse_f64(s: &str) -> Result<f64, std::num::ParseFloatError> {
    s.parse::<f64>()
}

/// Truncates to w columns; nothing for w < 0, like Go's truncate.
pub(crate) fn trunc(s: &str, w: i64) -> String {
    if w < 0 {
        String::new()
    } else {
        truncate(s, w as usize)
    }
}

fn clamp01(v: f64) -> f64 {
    0f64.max(1f64.min(v))
}

/// Draws a quadrant chart: a box split in four with each quadrant's
/// label, the points where they fall, and the axes' ends.
pub(crate) fn render_quadrant(_body: &str, rest: &str, opt: &Options) -> Drawn {
    struct Point {
        name: String,
        x: f64,
        y: f64,
    }
    let mut title = String::new();
    let (mut x_axis, mut y_axis) = ([String::new(), String::new()], [String::new(), String::new()]);
    let mut quads: HashMap<i64, String> = HashMap::new();
    let mut points: Vec<Point> = vec![];
    for raw in rest.split('\n') {
        let line = raw.trim();
        let (word, arg) = line.split_once(' ').unwrap_or((line, ""));
        let arg = arg.trim();
        if line.is_empty() {
        } else if word == "title" {
            title = arg.to_string();
        } else if word == "x-axis" || word == "y-axis" {
            let (lo, hi) = arg.split_once("-->").unwrap_or((arg, ""));
            let ends = [lo.trim().to_string(), hi.trim().to_string()];
            if word == "x-axis" {
                x_axis = ends;
            } else {
                y_axis = ends;
            }
        } else if let Some(n) = word.strip_prefix("quadrant-") {
            quads.insert(n.parse().unwrap_or(0), arg.to_string());
        } else if let Some(m) = QUAD_POINT.captures(line) {
            let x = parse_f64(&m[2]).unwrap_or(0.0);
            let y = parse_f64(&m[3]).unwrap_or(0.0);
            points.push(Point { name: m[1].trim_matches('"').to_string(), x: clamp01(x), y: clamp01(y) });
        }
    }
    if points.is_empty() && quads.is_empty() {
        return Err(Error::Empty);
    }
    let mut w = 64i64;
    if opt.width > 0 {
        w = 30.max(w.min(opt.width as i64 - 2));
    }
    let h = 12.max(w * 2 / 5) | 1; // odd, so the middle line is a row
    let left = if !y_axis[0].is_empty() || !y_axis[1].is_empty() { 2 } else { 0 };
    let mut cv = Canvas::new(left + w + 20, h + 3, opt.ascii);
    let x0 = left;
    cv.draw_box(x0, 0, w, h, LineStyle::Solid, false, false, Role::Cluster, "");
    let (mid_x, mid_y) = (x0 + w / 2, h / 2);
    cv.vline(mid_x, 0, h - 1, LineStyle::Dotted, Role::Cluster, "");
    cv.hline(x0, x0 + w - 1, mid_y, LineStyle::Dotted, Role::Cluster, "");
    // Mermaid numbers quadrants like the maths does: 1 top right, 2 top
    // left, 3 bottom left, 4 bottom right.
    let mut put = |q: i64, x1: i64, x2: i64, y: i64| {
        if let Some(t) = quads.get(&q).filter(|t| !t.is_empty()) {
            let t = trunc(t, x2 - x1 - 1);
            cv.text(x1 + (x2 - x1 - width(&t)) / 2, y, &t, Role::ClusterTitle, "", true);
        }
    };
    put(1, mid_x, x0 + w - 1, 1);
    put(2, x0, mid_x, 1);
    put(3, x0, mid_x, mid_y + 1);
    put(4, mid_x, x0 + w - 1, mid_y + 1);
    for p in &points {
        let x = x0 + 1 + (p.x * (w - 3) as f64 + 0.5) as i64;
        let y = 1 + ((1.0 - p.y) * (h - 3) as f64 + 0.5) as i64;
        cv.glyph_at(x, y, "●", Role::Bar, "");
        if x + 2 + width(&p.name) < x0 + w - 1 {
            cv.text(x + 2, y, &p.name, Role::NodeText, "", false);
        } else {
            cv.text(x - 1 - width(&p.name), y, &p.name, Role::NodeText, "", false);
        }
    }
    if !x_axis[0].is_empty() {
        cv.text(x0, h, &x_axis[0], Role::Muted, "", false);
    }
    if !x_axis[1].is_empty() {
        cv.text(x0 + w - width(&x_axis[1]), h, &x_axis[1], Role::Muted, "", false);
    }
    if !y_axis[1].is_empty() {
        cv.text(x0 + w + 1, 0, &format!("↑ {}", y_axis[1]), Role::Muted, "", false);
    }
    if !y_axis[0].is_empty() {
        cv.text(x0 + w + 1, h - 1, &format!("↓ {}", y_axis[0]), Role::Muted, "", false);
    }
    Ok((cv.picture(), title))
}

struct Series {
    bar: bool,
    vals: Vec<f64>,
}

/// Draws bar and line series over categories or a numeric x axis;
/// "xychart-beta horizontal" lays the bars sideways.
pub(crate) fn render_xychart(body: &str, rest: &str, opt: &Options) -> Drawn {
    let horizontal = body.trim().split('\n').next().unwrap_or("").contains("horizontal");
    let (mut title, mut x_title, mut y_title) = (String::new(), String::new(), String::new());
    let mut cats: Vec<String> = vec![];
    let (mut y_min, mut y_max) = (0f64, 0f64);
    let mut y_range = false;
    let mut all: Vec<Series> = vec![];
    for raw in rest.split('\n') {
        let line = raw.trim();
        let (word, arg) = line.split_once(' ').unwrap_or((line, ""));
        let arg = arg.trim();
        match word {
            "title" => title = arg.trim_matches('"').to_string(),
            "x-axis" => {
                if let Some(m) = AXIS_LIST.captures(arg) {
                    x_title = format!("{}{}", group(&m, 1), group(&m, 2)).trim().to_string();
                    for c in split_top_level(group(&m, 3), ',') {
                        cats.push(c.trim().trim_matches('"').to_string());
                    }
                } else if let Some(m) = AXIS_RANGE.captures(arg) {
                    x_title = format!("{}{}", group(&m, 1), group(&m, 2)).trim().to_string();
                }
            }
            "y-axis" => {
                if let Some(m) = AXIS_RANGE.captures(arg) {
                    y_title = format!("{}{}", group(&m, 1), group(&m, 2)).trim().to_string();
                    y_min = parse_f64(&m[3]).unwrap_or(0.0);
                    y_max = parse_f64(&m[4]).unwrap_or(0.0);
                    y_range = true;
                } else {
                    y_title = arg.trim_matches('"').to_string();
                }
            }
            _ => {
                if let Some(m) = SERIES_RE.captures(line) {
                    let mut s = Series { bar: &m[1] == "bar", vals: vec![] };
                    for v in m[2].split(',') {
                        if let Ok(f) = parse_f64(v.trim()) {
                            s.vals.push(f);
                        }
                    }
                    all.push(s);
                }
            }
        }
    }
    let n = all.iter().map(|s| s.vals.len()).max().unwrap_or(0);
    if n == 0 {
        return Err(Error::Empty);
    }
    while cats.len() < n {
        cats.push((cats.len() + 1).to_string());
    }
    if !y_range {
        (y_min, y_max) = (0.0, 0.0);
        for s in &all {
            for &v in &s.vals {
                (y_min, y_max) = (y_min.min(v), y_max.max(v));
            }
        }
        if y_max == y_min {
            y_max = y_min + 1.0;
        }
    }
    if horizontal {
        return Ok((xy_horizontal(&cats, &all, &x_title, &y_title, y_min, y_max, opt), title));
    }
    let n = n as i64;
    // Vertical: a value axis on the left, a column per category. The y
    // title heads the value axis; the x title sits under the categories.
    const H: i64 = 12;
    let top = if y_title.is_empty() { 0 } else { 1 };
    let n_bars = all.iter().filter(|s| s.bar).count() as i64;
    let mut col_w = 4.max((n_bars * 2).max(max_width(&cats) + 1));
    if opt.width > 0 {
        col_w = 2.max(col_w.min((opt.width as i64 - 10) / n));
    }
    let axis_w = width(&format_number(y_max)).max(width(&format_number(y_min))) + 1;
    let x0 = axis_w + 1;
    let mut rows = top + H + 2;
    if !x_title.is_empty() {
        rows += 1;
    }
    let mut cv = Canvas::new((x0 + n * col_w + 2).max(width(&y_title)), rows, opt.ascii);
    if !y_title.is_empty() {
        cv.text(0, 0, &y_title, Role::Muted, "", false);
    }
    // The axis with three values.
    cv.vline(x0 - 1, top, top + H, LineStyle::Solid, Role::Muted, "");
    cv.hline(x0 - 1, x0 + n * col_w, top + H, LineStyle::Solid, Role::Muted, "");
    for f in [0.0, 0.5, 1.0] {
        let v = y_min + f * (y_max - y_min);
        let y = H - 1 - (f * (H - 1) as f64 + 0.5) as i64;
        let lbl = format_number(v);
        cv.text(axis_w - width(&lbl), top + y, &lbl, Role::Muted, "", false);
    }
    let eighths: [&str; 9] =
        if opt.ascii { [" ", "#", "#", "#", "#", "#", "#", "#", "#"] } else { [" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"] };
    // In rows, fractional.
    let height = |v: f64| clamp01((v - y_min) / (y_max - y_min)) * H as f64;
    for (bi, s) in all.iter().filter(|s| s.bar).enumerate() {
        let bi = bi as i64;
        let bar_w = 1.max((col_w - 1) / 1.max(n_bars));
        for (i, &v) in s.vals.iter().enumerate() {
            let x = x0 + i as i64 * col_w + bi * bar_w;
            let mut units = (height(v) * 8.0 + 0.5) as i64;
            let mut row = 0;
            while units > 0 {
                let part = 8.min(units);
                cv.text(x, top + H - 1 - row, &eighths[part as usize].repeat(bar_w as usize), Role::Bar, GIT_COLORS[bi as usize % GIT_COLORS.len()], false);
                units -= part;
                row += 1;
            }
        }
    }
    for (li, s) in all.iter().enumerate() {
        if s.bar {
            continue;
        }
        let mut prev = (0i64, 0i64);
        for (i, &v) in s.vals.iter().enumerate() {
            let x = x0 + i as i64 * col_w + col_w / 2;
            let y = top + H - 1 - (height(v) * (H - 1) as f64 / H as f64 + 0.5) as i64;
            if i > 0 {
                draw_dots(&mut cv, prev, (x, y));
            }
            cv.text(x, y, "●", Role::Accent, GIT_COLORS[(li + 3) % GIT_COLORS.len()], true); // on top of the bars
            prev = (x, y);
        }
    }
    for (i, c) in cats.iter().enumerate() {
        cv.text(x0 + i as i64 * col_w, top + H + 1, &trunc(c, col_w - 1), Role::Muted, "", false);
    }
    if !x_title.is_empty() {
        let t = trunc(&x_title, n * col_w);
        cv.text(x0 + (n * col_w - width(&t)) / 2, top + H + 2, &t, Role::Muted, "", false);
    }
    Ok((cv.picture(), title))
}

/// Joins two points of a line series with dots.
fn draw_dots(cv: &mut Canvas, a: (i64, i64), b: (i64, i64)) {
    let steps = (b.0 - a.0).abs().max((b.1 - a.1).abs());
    for s in 1..steps {
        let x = a.0 + (b.0 - a.0) * s / steps;
        let y = a.1 + (b.1 - a.1) * s / steps;
        if cv.at(x, y).is_some_and(|c| c.text.is_empty() && c.glyph.is_empty()) {
            cv.glyph_at(x, y, "·", Role::Accent, "");
        }
    }
}

/// Draws the series sideways: a block of rows per category, one row for
/// each bar series, with line series as points joined across the blocks.
/// The x title heads the category names; the y title sits under the bars.
fn xy_horizontal(cats: &[String], all: &[Series], x_title: &str, y_title: &str, lo: f64, hi: f64, opt: &Options) -> Picture {
    let name_w = max_width(cats).max(width(x_title));
    let mut w = 50i64;
    if opt.width > 0 {
        w = 10.max(60.min(opt.width as i64 - name_w - 12));
    }
    let n_bars = all.iter().filter(|s| s.bar).count() as i64;
    let k = 1.max(n_bars);
    let gap = if k > 1 { 1 } else { 0 };
    let top = if x_title.is_empty() { 0 } else { 1 };
    let mut rows = top + cats.len() as i64 * (k + gap) - gap + 1;
    if !y_title.is_empty() {
        rows += 1;
    }
    let mut cv = Canvas::new(name_w + w + 14, rows, opt.ascii);
    if !x_title.is_empty() {
        cv.text(0, 0, x_title, Role::Muted, "", false);
    }
    let block = |i: i64| top + i * (k + gap);
    let frac = |v: f64| clamp01((v - lo) / (hi - lo));
    let eighths = ["", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"];
    for (i, c) in cats.iter().enumerate() {
        cv.text(0, block(i as i64), c, Role::NodeText, "", false);
    }
    for (bi, s) in all.iter().filter(|s| s.bar).enumerate() {
        for (i, &v) in s.vals.iter().enumerate().take(cats.len()) {
            let y = block(i as i64) + bi as i64;
            let units = (frac(v) * w as f64 * 8.0 + 0.5) as i64;
            let bar = if opt.ascii {
                "#".repeat(((units + 4) / 8) as usize)
            } else {
                format!("{}{}", "█".repeat((units / 8) as usize), eighths[(units % 8) as usize])
            };
            cv.text(name_w + 2, y, &bar, Role::Bar, GIT_COLORS[bi % GIT_COLORS.len()], false);
            cv.text(name_w + 3 + width(&bar), y, &format_number(v), Role::Muted, "", false);
        }
    }
    for (li, s) in all.iter().enumerate() {
        if s.bar {
            continue;
        }
        let mut prev = (0i64, 0i64);
        for (i, &v) in s.vals.iter().enumerate().take(cats.len()) {
            let x = name_w + 2 + (w - 1).min((frac(v) * (w - 1) as f64 + 0.5) as i64);
            let y = block(i as i64) + (k - 1) / 2;
            if i > 0 {
                draw_dots(&mut cv, prev, (x, y));
            }
            cv.text(x, y, "●", Role::Accent, GIT_COLORS[(li + 3) % GIT_COLORS.len()], true);
            prev = (x, y);
        }
    }
    if !y_title.is_empty() {
        let t = trunc(y_title, w);
        cv.text(name_w + 2 + (w - width(&t)) / 2, rows - 1, &t, Role::Muted, "", false);
    }
    cv.picture()
}
