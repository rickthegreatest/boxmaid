//! Sankey diagrams, radar charts and treemaps: in a terminal these read
//! best as proportional bars and nested boxes.

use std::collections::HashMap;

use crate::canvas::{Canvas, LineStyle};
use crate::charts::{parse_f64, trunc};
use crate::entities::format_number as fmt_num;
use crate::gitgraph::GIT_COLORS;
use crate::re::{group, lazy_re};
use crate::text::{or_str, pad_right, split_top_level, width};
use crate::{Drawn, Error, Options, Role};

struct Flow {
    from: String,
    to: String,
    v: f64,
}

/// Lists each node's outflows, in flow order (sources first), as bars
/// proportional to their value: the widths of a Sankey diagram, side by
/// side.
pub(crate) fn render_sankey(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut flows: Vec<Flow> = vec![];
    let mut order: HashMap<String, usize> = HashMap::new();
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let cols = split_csv(line);
        if cols.len() < 3 {
            continue;
        }
        let Ok(v) = parse_f64(cols[2].trim()) else { continue };
        let f = Flow { from: cols[0].trim().to_string(), to: cols[1].trim().to_string(), v };
        for n in [&f.from, &f.to] {
            if !order.contains_key(n) {
                order.insert(n.clone(), order.len());
            }
        }
        flows.push(f);
    }
    if flows.is_empty() {
        return Err(Error::Empty);
    }
    // Depth of each node: sources at 0, each flow one step further.
    let mut depth: HashMap<&str, i64> = HashMap::new();
    for _ in 0..order.len() {
        for f in &flows {
            let d = depth.get(f.from.as_str()).copied().unwrap_or(0) + 1;
            let e = depth.entry(f.to.as_str()).or_insert(0);
            *e = (*e).max(d);
        }
    }
    let mut sources: Vec<&str> = vec![];
    let mut out: HashMap<&str, Vec<&Flow>> = HashMap::new();
    let mut total: HashMap<&str, f64> = HashMap::new();
    let mut max_v = 0f64;
    for f in &flows {
        let o = out.entry(f.from.as_str()).or_default();
        if o.is_empty() {
            sources.push(&f.from);
        }
        o.push(f);
        *total.entry(f.from.as_str()).or_insert(0.0) += f.v;
        max_v = max_v.max(f.v);
    }
    let d = |s: &str| depth.get(s).copied().unwrap_or(0);
    sources.sort_by(|a, b| d(a).cmp(&d(b)).then(order[*a].cmp(&order[*b])));
    let name_w = order.keys().map(|n| width(n)).max().unwrap_or(0).min(28);
    let mut bar_w = 30i64;
    if opt.width > 0 {
        bar_w = 8.max(40.min(opt.width as i64 - 2 * name_w - 16));
    }
    let rows: i64 = sources.iter().map(|s| out[s].len() as i64 + 2).sum();
    let mut cv = Canvas::new(name_w * 2 + bar_w + 20, rows + 1, opt.ascii);
    let mut y = 0;
    let eighths = ["", "▏", "▎", "▍", "▌", "▋", "▊", "▉"];
    for (i, s) in sources.iter().enumerate() {
        let col = GIT_COLORS[i % GIT_COLORS.len()];
        cv.text(0, y, &trunc(s, name_w), Role::NodeText, col, true);
        cv.text(name_w + 2, y, &fmt_num(total[s]), Role::Muted, "", false);
        y += 1;
        for f in &out[s] {
            let units = (f.v / max_v * bar_w as f64 * 8.0 + 0.5) as i64;
            let mut bar = if opt.ascii {
                "#".repeat(1.max((units + 4) / 8) as usize)
            } else {
                format!("{}{}", "█".repeat((units / 8).max(0) as usize), eighths[units.rem_euclid(8) as usize])
            };
            if bar.is_empty() {
                bar = "▏".to_string();
            }
            cv.text(2, y, &bar, Role::Bar, col, false);
            cv.text(3 + width(&bar), y, &format!("{} → {}", fmt_num(f.v), f.to), Role::None, "", false);
            y += 1;
        }
        y += 1;
    }
    Ok((cv.picture(), String::new()))
}

/// Splits one CSV line, honouring double quotes.
fn split_csv(line: &str) -> Vec<String> {
    let b = line.as_bytes();
    let mut out = vec![];
    let mut cur: Vec<u8> = vec![];
    let mut quoted = false;
    let mut i = 0;
    while i < b.len() {
        let c = b[i];
        if c == b'"' && quoted && i + 1 < b.len() && b[i + 1] == b'"' {
            cur.push(b'"');
            i += 1;
        } else if c == b'"' {
            quoted = !quoted;
        } else if c == b',' && !quoted {
            out.push(String::from_utf8_lossy(&cur).into_owned());
            cur.clear();
        } else {
            cur.push(c);
        }
        i += 1;
    }
    out.push(String::from_utf8_lossy(&cur).into_owned());
    out
}

lazy_re!(RADAR_AXIS, r"^axis\s+(.+)$");
lazy_re!(RADAR_CURVE, r#"^curve\s+([\w-]+)(?:\["([^"]*)"\])?\s*\{(.*)\}\s*$"#);
lazy_re!(RADAR_ITEM, r#"^([\w-]+)(?:\["([^"]*)"\])?$"#);

struct Item {
    id: String,
    name: String,
}

/// Shows each axis with every curve's value as a bar, the values a radar
/// chart plots around its spokes.
pub(crate) fn render_radar(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let (mut axes, mut curves): (Vec<Item>, Vec<Item>) = (vec![], vec![]);
    let mut values: Vec<Vec<f64>> = vec![];
    let mut title = String::new();
    let mut max_v = 0f64;
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.starts_with("title ") {
            title = line["title".len()..].trim().trim_matches('"').to_string();
        } else if let Some(m) = RADAR_AXIS.captures(line) {
            for a in split_top_level(&m[1], ',') {
                if let Some(m) = RADAR_ITEM.captures(a.trim()) {
                    axes.push(Item { id: m[1].to_string(), name: or_str(group(&m, 2), &m[1]) });
                }
            }
        } else if line.starts_with("max ") {
            max_v = parse_f64(line["max".len()..].trim()).unwrap_or(0.0);
        } else if let Some(m) = RADAR_CURVE.captures(line) {
            curves.push(Item { id: m[1].to_string(), name: or_str(group(&m, 2), &m[1]) });
            let mut vals = vec![0f64; axes.len()];
            for (i, v) in split_top_level(&m[3], ',').iter().enumerate() {
                let v = v.trim();
                if let Some((k, val)) = v.split_once(':') {
                    // axis: value
                    for (j, a) in axes.iter().enumerate() {
                        if a.id == k.trim() {
                            vals[j] = parse_f64(val.trim()).unwrap_or(0.0);
                        }
                    }
                } else if i < vals.len() {
                    vals[i] = parse_f64(v).unwrap_or(0.0);
                }
            }
            values.push(vals);
        }
    }
    if axes.is_empty() || curves.is_empty() {
        return Err(Error::Empty);
    }
    if max_v == 0.0 {
        for vs in &values {
            for &v in vs {
                max_v = max_v.max(v);
            }
        }
    }
    max_v = max_v.max(1.0);
    let axis_w = axes.iter().map(|a| width(&a.name)).max().unwrap_or(0);
    let curve_w = curves.iter().map(|c| width(&c.name)).max().unwrap_or(0);
    let mut bar_w = 24i64;
    if opt.width > 0 {
        bar_w = 6.max(30.min(opt.width as i64 - axis_w - curve_w - 12));
    }
    let mut cv = Canvas::new(axis_w + curve_w + bar_w + 16, axes.len() as i64 * (curves.len() as i64 + 1) + 1, opt.ascii);
    let mut y = 0;
    for (ai, a) in axes.iter().enumerate() {
        cv.text(0, y, &a.name, Role::NodeText, "", true);
        for (ci, c) in curves.iter().enumerate() {
            let col = GIT_COLORS[ci % GIT_COLORS.len()];
            // A curve read before some axes has no value for them.
            let v = values[ci].get(ai).copied().unwrap_or(0.0);
            let n = (0f64.max(1f64.min(v / max_v)) * bar_w as f64 + 0.5) as i64;
            cv.text(axis_w + 2, y, &pad_right(&c.name, curve_w), Role::Muted, col, false);
            let bar = if opt.ascii { "#" } else { "█" }.repeat(n as usize);
            cv.text(axis_w + curve_w + 4, y, &bar, Role::Bar, col, false);
            cv.text(axis_w + curve_w + 5 + n, y, &fmt_num(v), Role::Muted, "", false);
            y += 1;
        }
        y += 1;
    }
    Ok((cv.picture(), title))
}

struct TNode {
    name: String,
    value: f64,
    indent: i64,
    children: Vec<usize>,
}

/// Draws the hierarchy as nested boxes whose areas follow the values, cut
/// alternately across and down (slice and dice).
pub(crate) fn render_treemap(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut nodes = vec![TNode { name: String::new(), value: 0.0, indent: -1, children: vec![] }];
    let mut stack = vec![0usize];
    let mut title = String::new();
    for raw in rest.split('\n') {
        if raw.trim().is_empty() {
            continue;
        }
        let t = raw.trim();
        if t.starts_with("title ") {
            title = t["title".len()..].trim().trim_matches('"').to_string();
            continue;
        }
        if t.starts_with("classDef ") || t.starts_with("%%") {
            continue;
        }
        let indent = (raw.len() - raw.trim_start_matches(' ').len()) as i64;
        let (name, val, has_val) = match t.split_once(':') {
            Some((n, v)) => (n, v, true),
            None => (t, "", false),
        };
        let val = match val.find(":::") {
            Some(i) => &val[..i],
            None => val,
        };
        let mut n = TNode { name: name.trim().trim_matches('"').to_string(), value: 0.0, indent, children: vec![] };
        if has_val {
            n.value = parse_f64(val.trim()).unwrap_or(0.0);
        }
        while stack.len() > 1 && nodes[*stack.last().unwrap()].indent >= indent {
            stack.pop();
        }
        let id = nodes.len();
        nodes.push(n);
        let p = *stack.last().unwrap();
        nodes[p].children.push(id);
        stack.push(id);
    }
    fn sum(nodes: &mut [TNode], n: usize) -> f64 {
        if nodes[n].children.is_empty() {
            return nodes[n].value.max(0.0);
        }
        let mut v = 0.0;
        for c in nodes[n].children.clone() {
            v += sum(nodes, c);
        }
        nodes[n].value = v;
        v
    }
    if nodes[0].children.is_empty() || sum(&mut nodes, 0) <= 0.0 {
        return Err(Error::Empty);
    }
    let w = if opt.width > 0 { 20.max(opt.width as i64) } else { 80 };
    let h = 8.max(w / 4); // cells are about twice as tall as wide
    let mut cv = Canvas::new(w, h, opt.ascii);
    #[allow(clippy::too_many_arguments)]
    fn place(cv: &mut Canvas, nodes: &[TNode], list: &[usize], x: i64, y: i64, w: i64, h: i64, across: bool, depth: usize) {
        let total: f64 = list.iter().map(|&n| nodes[n].value).sum();
        if total <= 0.0 || w < 3 || h < 2 {
            return;
        }
        let mut pos = 0f64;
        for (i, &ni) in list.iter().enumerate() {
            let n = &nodes[ni];
            let share = n.value / total;
            let (bx, by, bw, bh);
            if across {
                bx = x + (pos * w as f64 + 0.5) as i64;
                let mut end = x + ((pos + share) * w as f64 + 0.5) as i64;
                if i == list.len() - 1 {
                    end = x + w;
                }
                (by, bw, bh) = (y, end - bx, h);
            } else {
                by = y + (pos * h as f64 + 0.5) as i64;
                let mut end = y + ((pos + share) * h as f64 + 0.5) as i64;
                if i == list.len() - 1 {
                    end = y + h;
                }
                (bx, bw, bh) = (x, w, end - by);
            }
            pos += share;
            if bw < 2 || bh < 2 {
                continue;
            }
            let parent = !n.children.is_empty();
            let role = if parent { Role::Cluster } else { Role::Node };
            cv.draw_box(bx, by, bw, bh, LineStyle::Solid, parent, false, role, GIT_COLORS[depth % GIT_COLORS.len()]);
            let mut lbl = n.name.clone();
            if !parent {
                lbl = format!("{lbl} {}", fmt_num(n.value));
            }
            if bw > 4 {
                cv.text(bx + 1, by, &trunc(&format!(" {lbl} "), bw - 2), Role::ClusterTitle, "", parent);
            }
            if parent {
                place(cv, nodes, &n.children, bx + 1, by + 1, bw - 2, bh - 2, !across, depth + 1);
            }
        }
    }
    let top = nodes[0].children.clone();
    place(&mut cv, &nodes, &top, 0, 0, w, h, true, 0);
    Ok((cv.picture(), title))
}
