//! Timelines: periods down a spine, each with its events beside it, grouped
//! under their sections.
//!
//! ```text
//! 2002 ─┬─ LinkedIn
//!       │
//! 2004 ─┼─ Facebook
//!       │  Google
//! ```

use crate::canvas::{Canvas, LineStyle};
use crate::text::{width, wrap};
use crate::{Drawn, Error, Options, Role};

struct Period {
    name: String,
    section: String,
    events: Vec<String>,
}

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let (mut title, mut section) = (String::new(), String::new());
    let mut periods: Vec<Period> = vec![];
    for raw in rest.split('\n') {
        let line = raw.trim();
        let (word, arg) = line.split_once(' ').unwrap_or((line, ""));
        match word {
            "" => continue,
            "title" => {
                title = arg.trim().to_string();
                continue;
            }
            "section" => {
                section = arg.trim().to_string();
                continue;
            }
            "accTitle" | "accTitle:" | "accDescr" | "accDescr:" => continue,
            _ => {}
        }
        let mut parts = line.split(':');
        let head = parts.next().unwrap_or("").trim().to_string();
        let events: Vec<String> = parts.map(str::trim).filter(|e| !e.is_empty()).map(String::from).collect();
        if head.is_empty() {
            if let Some(p) = periods.last_mut() {
                // ": another event" continues the period above.
                p.events.extend(events);
                continue;
            }
        }
        periods.push(Period { name: head, section: section.clone(), events });
    }
    if periods.is_empty() {
        return Err(Error::Empty);
    }
    let name_w = periods.iter().map(|p| width(&p.name)).max().unwrap_or(0);
    let ev_w = if opt.width > 0 { (opt.width as i64 - name_w - 5).max(16) } else { 60 };
    let mut lines = 0i64;
    let mut last: Option<&str> = None;
    for p in &periods {
        if last != Some(p.section.as_str()) {
            lines += 2;
            last = Some(&p.section);
        }
        for e in &p.events {
            lines += wrap(std::slice::from_ref(e), ev_w).len() as i64;
        }
        lines += 2;
    }
    let mut cv = Canvas::new(name_w + ev_w + 6, lines + 2, opt.ascii);
    let spine = name_w + 2;
    let mut y = 0i64;
    let mut prev_y = -1i64; // the previous period's row in this section: the spine joins them
    let mut last: Option<&str> = None;
    for p in &periods {
        if last != Some(p.section.as_str()) {
            last = Some(&p.section);
            prev_y = -1;
            if !p.section.is_empty() {
                cv.text(0, y, &p.section, Role::ClusterTitle, "", true);
                y += 1;
            }
        }
        cv.text(name_w - width(&p.name), y, &p.name, Role::NodeText, "", true);
        cv.hline(name_w + 1, spine + 2, y, LineStyle::Solid, Role::Edge, "");
        if prev_y >= 0 {
            cv.vline(spine, prev_y, y, LineStyle::Solid, Role::Edge, "");
        }
        prev_y = y;
        let mut ey = y;
        for e in &p.events {
            for l in wrap(std::slice::from_ref(e), ev_w) {
                cv.text(spine + 4, ey, &l, Role::None, "", false);
                ey += 1;
            }
        }
        y = (y + 1).max(ey) + 1;
    }
    Ok((cv.picture(), title))
}
