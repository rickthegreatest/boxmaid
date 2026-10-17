//! Kanban boards and packet diagrams.

use crate::canvas::{Canvas, LineStyle};
use crate::charts::trunc;
use crate::re::{group, lazy_re};
use crate::text::{label, width, wrap};
use crate::{Drawn, Error, Options, Role};

lazy_re!(KANBAN_ITEM, r"^([\w-]+)?\s*(?:\[(.*?)\])?\s*(?:@\{(.*)\})?\s*$");
lazy_re!(KANBAN_META, r#"(\w+)\s*:\s*('[^']*'|"[^"]*"|[^,}]+)"#);
lazy_re!(PACKET_ROW, r#"^(?:(\d+)(?:\s*-\s*(\d+))?|\+(\d+))\s*:\s*"([^"]*)""#);

struct Card {
    text: Vec<String>,
    meta: String,
    prio: String,
}

struct Column {
    title: String,
    cards: Vec<Card>,
}

/// Draws the columns side by side, each with its cards (and their ticket,
/// assignee and priority); columns that don't fit the width wrap to a new
/// row of columns.
pub(crate) fn render_kanban(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut cols: Vec<Column> = vec![];
    let mut col_indent: Option<usize> = None;
    for raw in rest.split('\n') {
        if raw.trim().is_empty() {
            continue;
        }
        let indent = raw.len() - raw.trim_start_matches(' ').len();
        let Some(m) = KANBAN_ITEM.captures(raw.trim()) else { continue };
        let text = if group(&m, 2).is_empty() { group(&m, 1) } else { group(&m, 2) };
        if col_indent.is_none_or(|ci| indent <= ci) {
            col_indent = Some(indent);
            cols.push(Column { title: text.to_string(), cards: vec![] });
            continue;
        }
        let mut c = Card { text: label(text), meta: String::new(), prio: String::new() };
        let mut meta: Vec<String> = vec![];
        for kv in KANBAN_META.captures_iter(group(&m, 3)) {
            let v = kv[2].trim().trim_matches(|c| c == '\'' || c == '"').to_string();
            match &kv[1] {
                "priority" => c.prio = v,
                "assigned" => meta.push(format!("@{v}")),
                _ => meta.push(v),
            }
        }
        c.meta = meta.join(" · ");
        cols.last_mut().unwrap().cards.push(c);
    }
    if cols.is_empty() {
        return Err(Error::Empty);
    }
    let w = if opt.width > 0 { opt.width as i64 } else { 120 };
    let mut col_w = 26i64;
    let mut per_row = 1.max((w + 1) / (col_w + 1));
    let nc = cols.len() as i64;
    if nc < per_row {
        col_w = 36.min(col_w.max((w + 1) / nc - 1));
        per_row = nc;
    }
    let inner = col_w - 6;
    // Height of each band of columns.
    let card_h = |c: &Card| -> i64 {
        let mut h = wrap(&c.text, inner).len() as i64 + 2;
        if !c.meta.is_empty() || !c.prio.is_empty() {
            h += 1;
        }
        h
    };
    let bands: Vec<&[Column]> = cols.chunks(per_row as usize).collect();
    let mut total = 0;
    let mut band_h = vec![0i64; bands.len()];
    for (bi, band) in bands.iter().enumerate() {
        for c in band.iter() {
            let h = 2 + c.cards.iter().map(card_h).sum::<i64>();
            band_h[bi] = band_h[bi].max(h + 1);
        }
        total += band_h[bi] + 1;
    }
    let mut cv = Canvas::new(per_row * (col_w + 1) + 1, total + 1, opt.ascii);
    let mut y0 = 0;
    for (bi, band) in bands.iter().enumerate() {
        for (ci, c) in band.iter().enumerate() {
            let x = ci as i64 * (col_w + 1);
            cv.draw_box(x, y0, col_w, band_h[bi], LineStyle::Solid, true, false, Role::Cluster, "");
            cv.text(x + 2, y0, &format!(" {} ", trunc(&c.title, col_w - 6)), Role::ClusterTitle, "", true);
            let count = c.cards.len().to_string();
            cv.text(x + col_w - 2 - width(&count), y0, &count, Role::Muted, "", false);
            let mut y = y0 + 1;
            for cd in &c.cards {
                let h = card_h(cd);
                cv.draw_box(x + 1, y, col_w - 2, h, LineStyle::Solid, false, false, Role::Node, "");
                for (i, l) in wrap(&cd.text, inner).iter().enumerate() {
                    cv.text(x + 3, y + 1 + i as i64, l, Role::NodeText, "", false);
                }
                if !cd.meta.is_empty() || !cd.prio.is_empty() {
                    if !cd.prio.is_empty() {
                        cv.text(x + col_w - 4 - width(&cd.prio), y + h - 2, &cd.prio, priority_role(&cd.prio), "", true);
                    }
                    cv.text(x + 3, y + h - 2, &trunc(&cd.meta, inner - width(&cd.prio) - 1), Role::Muted, "", false);
                }
                y += h;
            }
        }
        y0 += band_h[bi] + 1;
    }
    Ok((cv.picture(), String::new()))
}

fn priority_role(p: &str) -> Role {
    match p.to_lowercase().as_str() {
        "very high" | "high" => Role::Warn,
        "low" | "very low" => Role::Good,
        _ => Role::Accent,
    }
}

/// Draws a packet's fields as boxes on rows of 32 bits, with the bit
/// numbers above each row.
pub(crate) fn render_packet(_body: &str, rest: &str, opt: &Options) -> Drawn {
    struct Field {
        lo: i64,
        hi: i64,
        name: String,
    }
    let mut fields: Vec<Field> = vec![];
    let mut title = String::new();
    let mut next = 0i64;
    let atoi = |s: &str| s.parse::<i64>().unwrap_or(0);
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.starts_with("title ") {
            title = line["title".len()..].trim().trim_matches('"').to_string();
            continue;
        }
        let Some(m) = PACKET_ROW.captures(line) else { continue };
        let (lo, hi);
        if !group(&m, 3).is_empty() {
            // +n: the next n bits
            let n = atoi(group(&m, 3));
            (lo, hi) = (next, next + 1.max(n) - 1);
        } else {
            lo = atoi(group(&m, 1));
            hi = if group(&m, 2).is_empty() { lo } else { atoi(group(&m, 2)) };
        }
        fields.push(Field { lo, hi, name: group(&m, 4).to_string() });
        next = hi + 1;
    }
    if fields.is_empty() {
        return Err(Error::Empty);
    }
    const BITS_PER_ROW: i64 = 32;
    let mut cell_w = 3i64;
    if opt.width > 0 {
        cell_w = 1.max(3.min((opt.width as i64 - 1) / BITS_PER_ROW));
    }
    let rows = (next + BITS_PER_ROW - 1) / BITS_PER_ROW;
    let mut cv = Canvas::new(BITS_PER_ROW * cell_w + 2, rows * 4 + 1, opt.ascii);
    for r in 0..rows {
        let y = r * 4;
        for f in &fields {
            let (lo, hi) = (f.lo.max(r * BITS_PER_ROW), f.hi.min(r * BITS_PER_ROW + BITS_PER_ROW - 1));
            if lo > hi {
                continue;
            }
            let x1 = (lo - r * BITS_PER_ROW) * cell_w;
            let x2 = (hi - r * BITS_PER_ROW + 1) * cell_w;
            cv.draw_box(x1, y + 1, x2 - x1 + 1, 3, LineStyle::Solid, false, false, Role::Node, "");
            let name = trunc(&f.name, x2 - x1 - 1);
            cv.text(x1 + (x2 - x1 + 1 - width(&name)) / 2, y + 2, &name, Role::NodeText, "", false);
            // Bit numbers just inside the field's edges, so neighbours'
            // numbers never run together.
            let (los, end) = (lo.to_string(), hi.to_string());
            cv.text(x1 + 1, y, &los, Role::Muted, "", false);
            if hi != lo && x2 - width(&end) > x1 + 1 + width(&los) {
                cv.text(x2 - width(&end), y, &end, Role::Muted, "", false);
            }
        }
    }
    Ok((cv.picture(), title))
}
