//! User journeys: each section, then its tasks with their score (1–5) as a
//! bar coloured from bad to good, and who takes part.

use crate::canvas::Canvas;
use crate::re::lazy_re;
use crate::text::width;
use crate::width::truncate;
use crate::{Drawn, Error, Options, Role};

lazy_re!(JOURNEY_TASK, r"^(.+?)\s*:\s*(\d+)\s*(?::\s*(.*))?$");

struct Task {
    name: String,
    section: String,
    score: i64,
    actors: String,
}

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let (mut title, mut section) = (String::new(), String::new());
    let mut tasks: Vec<Task> = vec![];
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
        if let Some(m) = JOURNEY_TASK.captures(line) {
            let n: i64 = m[2].parse().unwrap_or(0);
            let actors = m.get(3).map_or("", |a| a.as_str()).trim().to_string();
            tasks.push(Task { name: m[1].to_string(), section: section.clone(), score: n.clamp(0, 5), actors });
        }
    }
    if tasks.is_empty() {
        return Err(Error::Empty);
    }
    let mut name_w = tasks.iter().map(|t| width(&t.name)).fold(6, i64::max);
    if opt.width > 0 {
        name_w = name_w.min((opt.width as i64 - 24).max(10));
    }
    let mut rows = tasks.len() as i64;
    let mut last: Option<&str> = None;
    for t in &tasks {
        if last != Some(t.section.as_str()) {
            rows += 2;
            last = Some(&t.section);
        }
    }
    let mut cv = Canvas::new(name_w + 80, rows + 1, opt.ascii);
    let mut y = 0;
    let mut last: Option<&str> = None;
    for t in &tasks {
        if last != Some(t.section.as_str()) {
            if last.is_some() {
                y += 1;
            }
            last = Some(&t.section);
            if !t.section.is_empty() {
                cv.text(0, y, &t.section, Role::ClusterTitle, "", true);
                y += 1;
            }
        }
        cv.text(2, y, &truncate(&t.name, name_w.max(0) as usize), Role::NodeText, "", false);
        let role = match t.score {
            s if s <= 2 => Role::Warn,
            3 => Role::Accent,
            _ => Role::Good,
        };
        let x = name_w + 4;
        let (filled, empty) = if opt.ascii { ("#", ".") } else { ("●", "○") };
        cv.text(x, y, &filled.repeat(t.score as usize), role, "", false);
        cv.text(x + t.score, y, &empty.repeat((5 - t.score) as usize), Role::Muted, "", false);
        cv.text(x + 6, y, &t.score.to_string(), role, "", true);
        if !t.actors.is_empty() {
            cv.text(x + 9, y, &t.actors, Role::Muted, "", false);
        }
        y += 1;
    }
    Ok((cv.picture(), title))
}
