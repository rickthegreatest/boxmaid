//! Gantt charts: one row per task under its section, a bar from start to
//! end on a shared time axis. Done tasks are muted, active ones accented,
//! critical ones in the warning colour; milestones are a ◆. Starts may be
//! dates or "after" other tasks, ends dates, durations (3d, 1w, 12h) or
//! "until" another task; "excludes weekends" stretches durations over them.

use std::collections::HashMap;

use crate::canvas::{Canvas, LineStyle};
use crate::dates::{add_months, format_axis_date, parse_moment_date, weekday, MS_DAY, MS_HOUR, MS_MINUTE, MS_SECOND};
use crate::re::lazy_re;
use crate::text::width;
use crate::width::truncate;
use crate::{Drawn, Error, Options, Role};

#[derive(Default)]
struct Task {
    name: String,
    id: String,
    section: String,
    done: bool,
    active: bool,
    crit: bool,
    mile: bool,
    /// Milliseconds since 1970, UTC.
    start: i64,
    end: i64,
    start_spec: String,
    end_spec: String,
    resolved: bool,
}

lazy_re!(GANTT_TASK, r"^(.+?)\s*:\s*(.*)$");
lazy_re!(GANTT_DUR, r"^(\d+(?:\.\d+)?)\s*(ms|s|m|h|d|w|M|y)$");
lazy_re!(GANTT_AFTER, r"^after\s+(.+)$");
lazy_re!(GANTT_UNTIL, r"^until\s+(.+)$");

fn parse_gantt_date(s: &str, format: &str) -> Option<i64> {
    let s = s.trim();
    [format, "YYYY-MM-DD", "YYYY-MM-DD HH:mm", "YYYY-MM-DDTHH:mm:ss"].into_iter().find_map(|f| parse_moment_date(s, f))
}

/// Adds a Mermaid duration ("3d", "1w", "12h") to t, skipping weekends for
/// day-based durations when asked.
fn add_duration(mut t: i64, spec: &str, skip_weekends: bool) -> Option<i64> {
    let m = GANTT_DUR.captures(spec.trim())?;
    let v: f64 = m[1].parse().unwrap_or(0.0);
    let u = &m[2];
    let unit = match u {
        "ms" => 1,
        "s" => MS_SECOND,
        "m" => MS_MINUTE,
        "h" => MS_HOUR,
        "d" => MS_DAY,
        "w" => 7 * MS_DAY,
        "M" => return Some(add_months(t, v as i64)),
        _ => return Some(add_months(t, 12 * v as i64)),
    };
    if skip_weekends && (u == "d" || u == "w") {
        let mut days = v as i64;
        if u == "w" {
            days *= 5;
        }
        while days > 0 {
            t += MS_DAY;
            let wd = weekday(t);
            if wd != 0 && wd != 6 {
                days -= 1;
            }
        }
        return Some(t);
    }
    Some(t + (v * unit as f64) as i64)
}

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut layout = "YYYY-MM-DD".to_string();
    let (mut title, mut section) = (String::new(), String::new());
    let mut skip_weekends = false;
    let mut tasks: Vec<Task> = vec![];
    for raw in rest.split('\n') {
        let line = raw.trim();
        let (word, arg) = line.split_once(' ').unwrap_or((line, ""));
        let arg = arg.trim();
        match word {
            "" => continue,
            "title" => {
                title = arg.to_string();
                continue;
            }
            "dateFormat" => {
                layout = arg.to_string();
                continue;
            }
            "section" => {
                section = arg.to_string();
                continue;
            }
            "excludes" => {
                skip_weekends = arg.to_lowercase().contains("weekend");
                continue;
            }
            "axisFormat" | "tickInterval" | "todayMarker" | "includes" | "weekday" | "inclusiveEndDates" | "topAxis" | "accTitle" | "accTitle:"
            | "accDescr" | "accDescr:" | "click" | "displayMode" => continue,
            _ => {}
        }
        let Some(m) = GANTT_TASK.captures(line) else { continue };
        let mut t = Task { name: m[1].to_string(), section: section.clone(), ..Default::default() };
        let mut meta: Vec<String> = vec![];
        for p in m[2].split(',') {
            match p.trim() {
                "done" => t.done = true,
                "active" => t.active = true,
                "crit" => t.crit = true,
                "milestone" => t.mile = true,
                "" => {}
                p => meta.push(p.to_string()),
            }
        }
        match meta.len() {
            0 => {}
            1 => t.end_spec = meta[0].clone(),
            2 => (t.start_spec, t.end_spec) = (meta[0].clone(), meta[1].clone()),
            _ => (t.id, t.start_spec, t.end_spec) = (meta[0].clone(), meta[1].clone(), meta[2].clone()),
        }
        tasks.push(t);
    }
    if tasks.is_empty() {
        return Err(Error::Empty);
    }
    let mut by_id: HashMap<String, usize> = HashMap::new();
    for (i, t) in tasks.iter().enumerate() {
        if !t.id.is_empty() {
            by_id.insert(t.id.clone(), i);
        }
    }
    // Resolve starts and ends in order; "after" may refer forward, so
    // repeat until nothing changes.
    for _ in 0..tasks.len() + 1 {
        for i in 0..tasks.len() {
            if !tasks[i].resolved {
                resolve_task(&mut tasks, i, &by_id, &layout, skip_weekends);
            }
        }
    }
    let (mut lo, mut hi) = (0i64, 0i64);
    let mut found = false;
    for t in tasks.iter().filter(|t| t.resolved) {
        if !found || t.start < lo {
            lo = t.start;
        }
        if !found || t.end > hi {
            hi = t.end;
        }
        found = true;
    }
    if !found {
        return Err(Error::Empty);
    }
    if hi <= lo {
        hi = lo + MS_DAY;
    }
    // Columns: task names, then the chart.
    let w = if opt.width > 0 { opt.width as i64 } else { 100 };
    let name_w = tasks.iter().map(|t| width(&t.name).min(28)).fold(8, i64::max);
    let chart_w = (w - name_w - 3).max(20);
    let span = hi - lo;
    let col = |t: i64| ((chart_w - 1) as f64 * (t - lo) as f64 / span as f64) as i64;
    let mut rows = 2i64; // axis
    let mut sections = 0i64;
    let mut last: Option<&str> = None;
    for t in &tasks {
        if last != Some(t.section.as_str()) {
            sections += 1;
            last = Some(&t.section);
        }
        rows += 1;
    }
    let mut cv = Canvas::new(name_w + 3 + chart_w, rows + sections + 1, opt.ascii);
    let x0 = name_w + 3;
    // Axis: dates at even steps, as many as fit.
    cv.hline(x0, x0 + chart_w - 1, 1, LineStyle::Solid, Role::Muted, "");
    let day_fmt = if span < 2 * MS_DAY {
        "time"
    } else if span > 400 * MS_DAY {
        "month"
    } else {
        "day"
    };
    let label_w = width(&format_axis_date(lo, day_fmt)) + 3;
    let ticks = (chart_w / label_w).max(1);
    let mut free = 0; // the first column a label may start at
    for i in 0..=ticks {
        let x = x0 + i * (chart_w - 1) / ticks;
        let lbl = format_axis_date(lo + (span as f64 * i as f64 / ticks as f64) as i64, day_fmt);
        let lx = x.min(x0 + chart_w - width(&lbl));
        if lx < free {
            continue; // it would run into the previous date
        }
        cv.text(lx, 0, &lbl, Role::Muted, "", false);
        cv.glyph_at(x, 1, "┬", Role::Muted, "");
        free = lx + width(&lbl) + 2;
    }
    let mut y = 2i64;
    let mut last: Option<&str> = None;
    for t in &tasks {
        if last != Some(t.section.as_str()) {
            last = Some(&t.section);
            if !t.section.is_empty() {
                cv.text(0, y, &truncate(&t.section, (name_w + 2) as usize), Role::ClusterTitle, "", true);
                y += 1;
            }
        }
        cv.text(1, y, &truncate(&t.name, name_w as usize), Role::NodeText, "", false);
        if !t.resolved {
            cv.text(x0, y, "?", Role::Muted, "", false);
            y += 1;
            continue;
        }
        let (role, bar) = if t.crit {
            (Role::Warn, "█")
        } else if t.active {
            (Role::Accent, "█")
        } else if t.done {
            (Role::Muted, "▓")
        } else {
            (Role::Bar, "█")
        };
        let (a, b) = (col(t.start), col(t.end));
        if t.mile || t.end <= t.start {
            cv.text(x0 + a, y, "◆", role, "", false);
        } else {
            cv.text(x0 + a, y, &bar.repeat((b - a).max(1) as usize), role, "", false);
        }
        y += 1;
    }
    Ok((cv.picture(), title))
}

fn resolve_task(tasks: &mut [Task], i: usize, by_id: &HashMap<String, usize>, layout: &str, skip: bool) {
    // Start: a date, "after a b" (the latest end), or right after the
    // previous task.
    let t = &tasks[i];
    let start = if t.start_spec.is_empty() {
        if i == 0 || !tasks[i - 1].resolved {
            return;
        }
        tasks[i - 1].end
    } else if let Some(m) = GANTT_AFTER.captures(&t.start_spec) {
        let mut start = -1i64 << 62; // the latest end of the tasks named
        for id in m[1].split_whitespace() {
            match by_id.get(id) {
                Some(&o) if tasks[o].resolved => start = start.max(tasks[o].end),
                _ => return,
            }
        }
        start
    } else if let Some(d) = parse_gantt_date(&t.start_spec, layout) {
        d
    } else {
        return;
    };
    // End: a date, a duration, or "until a" (that task's start).
    let mut end = start;
    if let Some(m) = GANTT_UNTIL.captures(&t.end_spec) {
        match by_id.get(m[1].trim()) {
            Some(&o) if tasks[o].resolved => end = tasks[o].start,
            _ => return,
        }
    } else if let Some(e) = add_duration(start, &t.end_spec, skip) {
        end = e;
    } else if let Some(d) = parse_gantt_date(&t.end_spec, layout) {
        end = d;
    }
    let t = &mut tasks[i];
    (t.start, t.end, t.resolved) = (start, end, true);
}
