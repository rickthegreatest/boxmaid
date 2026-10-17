//! Git graphs: each branch is a lane, commits sit along it in order, a new
//! branch leaves its parent where it was created and a merge comes back in.
//!
//! ```text
//! main     ●───●───────────◉───●
//!              │           │
//! develop      ╰───●───●───╯
//! ```
//!
//! Commits are ● (■ highlighted, ⊗ reverted, ◎ cherry-picked, ◉ merges);
//! tags are written above them. "gitGraph TB:" runs top to bottom.

use std::collections::HashMap;

use crate::canvas::{Canvas, LineStyle};
use crate::re::lazy_re;
use crate::text::width;
use crate::{Drawn, Error, Options, Picture, Role};

/// Colours for lanes and series, in order.
pub(crate) const GIT_COLORS: [&str; 8] = ["#4f9cff", "#3ddc84", "#ffb547", "#d58cff", "#ff6b6b", "#4fd6d6", "#ff8fc7", "#a3e635"];

lazy_re!(GIT_OPT, r#"(\w+)\s*:\s*("[^"]*"|\S+)"#);

#[derive(Default)]
struct Commit {
    id: String,
    tag: String,
    kind: String,
    branch: usize,
    parents: Vec<usize>,
    merge: bool,
    cherry: bool,
}

struct Branch {
    name: String,
    order: i64,
    /// The last commit on it (or the one it branched from).
    head: Option<usize>,
    /// Its first own commit.
    first: Option<usize>,
}

fn opts(s: &str) -> HashMap<String, String> {
    let mut m = HashMap::new();
    for o in GIT_OPT.captures_iter(s) {
        m.insert(o[1].to_string(), o[2].trim_matches('"').to_string());
    }
    m
}

fn get<'a>(m: &'a HashMap<String, String>, k: &str) -> &'a str {
    m.get(k).map_or("", String::as_str)
}

pub(crate) fn render(body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut dir = "LR";
    let first_line = body.trim().split('\n').next().unwrap_or("");
    let h: Vec<&str> = first_line.split_whitespace().collect();
    if h.len() > 1 {
        match h[1].strip_suffix(':').unwrap_or(h[1]) {
            "TB" => dir = "TB",
            "BT" => dir = "BT",
            _ => {}
        }
    }
    let mut branches = vec![Branch { name: "main".into(), order: 0, head: None, first: None }];
    let mut by_name: HashMap<String, usize> = HashMap::from([("main".to_string(), 0)]);
    let mut cur = 0usize;
    let mut commits: Vec<Commit> = vec![];
    let add = |c: Commit, cur: usize, branches: &mut Vec<Branch>, commits: &mut Vec<Commit>| {
        let mut c = c;
        c.branch = cur;
        let b = &mut branches[cur];
        if let Some(h) = b.head {
            c.parents.insert(0, h);
        }
        commits.push(c);
        b.head = Some(commits.len() - 1);
        if b.first.is_none() {
            b.first = b.head;
        }
    };
    for raw in rest.split('\n') {
        let line = raw.trim();
        let (word, args) = line.split_once(' ').unwrap_or((line, ""));
        let o = opts(args);
        match word {
            "commit" => {
                let c = Commit { id: get(&o, "id").into(), tag: get(&o, "tag").into(), kind: get(&o, "type").to_uppercase(), ..Default::default() };
                add(c, cur, &mut branches, &mut commits);
            }
            "branch" => {
                let name = args.split_whitespace().next().unwrap_or("");
                if name.is_empty() || by_name.contains_key(name) {
                    continue;
                }
                let order = get(&o, "order").parse::<i64>().unwrap_or(branches.len() as i64);
                let head = branches[cur].head;
                branches.push(Branch { name: name.into(), order, head, first: None });
                by_name.insert(name.into(), branches.len() - 1);
                cur = branches.len() - 1;
            }
            "checkout" | "switch" => {
                if let Some(&i) = by_name.get(args.trim()) {
                    cur = i;
                }
            }
            "merge" => {
                let name = args.split_whitespace().next().unwrap_or("");
                let Some(&i) = by_name.get(name) else { continue };
                let Some(h) = branches[i].head else { continue };
                let c = Commit {
                    id: get(&o, "id").into(),
                    tag: get(&o, "tag").into(),
                    kind: get(&o, "type").to_uppercase(),
                    merge: true,
                    parents: vec![h],
                    ..Default::default()
                };
                add(c, cur, &mut branches, &mut commits);
            }
            "cherry-pick" => {
                let mut c = Commit { tag: get(&o, "tag").into(), cherry: true, ..Default::default() };
                if !get(&o, "id").is_empty() {
                    c.tag = format!("{} ← {}", c.tag, get(&o, "id")).trim().to_string();
                }
                add(c, cur, &mut branches, &mut commits);
            }
            _ => {}
        }
    }
    if commits.is_empty() {
        return Err(Error::Empty);
    }
    // Lanes in order: main first, then by "order:" and creation.
    let mut lanes = vec![0i64; branches.len()]; // branch → lane
    let mut idx: Vec<usize> = (0..branches.len()).collect();
    for i in 1..idx.len() {
        let mut j = i;
        while j > 1 && branches[idx[j]].order < branches[idx[j - 1]].order {
            idx.swap(j, j - 1);
            j -= 1;
        }
    }
    for (lane, &b) in idx.iter().enumerate() {
        lanes[b] = lane as i64;
    }
    let name_w = branches.iter().map(|b| width(&b.name)).max().unwrap_or(0);
    // Geometry: t runs along the history, lane across it.
    const STEP: i64 = 4;
    let tb = dir != "LR";
    let lane_gap = if tb { 4.max(name_w + 2) } else { 3 };
    let pos = |t: i64, lane: i64| -> (i64, i64) {
        if tb {
            (1 + lane * lane_gap, 2 + t * 2)
        } else {
            (name_w + 3 + t * STEP, 1 + lane * lane_gap)
        }
    };
    let (nb, nc) = (branches.len() as i64, commits.len() as i64);
    let (w, h) = if tb { (2 + nb * lane_gap, 4 + nc * 2) } else { (name_w + 6 + nc * STEP, 3 + nb * lane_gap) };
    let mut cv = Canvas::new(w, h, opt.ascii);
    let line = |cv: &mut Canvas, t1: i64, l1: i64, t2: i64, l2: i64, color: &str| {
        let (x1, y1) = pos(t1, l1);
        let (x2, y2) = pos(t2, l2);
        if x1 == x2 {
            cv.vline(x1, y1, y2, LineStyle::Solid, Role::Edge, color);
        } else {
            cv.hline(x1, x2, y1, LineStyle::Solid, Role::Edge, color);
        }
    };
    let color = |b: usize| GIT_COLORS[lanes[b] as usize % GIT_COLORS.len()];
    // Branch lines, forks and merges (forks and merges turn where the lane
    // changes, so they read as curves).
    for (bi, b) in branches.iter().enumerate() {
        let Some(first) = b.first else { continue };
        let lane = lanes[bi];
        let mut last = first;
        for (i, c) in commits.iter().enumerate() {
            if c.branch == bi {
                last = i;
            }
        }
        line(&mut cv, first as i64, lane, last as i64, lane, color(bi));
        if let Some(&from) = commits[first].parents.first() {
            let pl = lanes[commits[from].branch];
            line(&mut cv, from as i64, pl, from as i64, lane, color(bi));
            line(&mut cv, from as i64, lane, first as i64, lane, color(bi));
            let (x, y) = pos(from as i64, lane);
            if let Some(c) = cv.at(x, y) {
                c.round = true;
            }
        }
    }
    for (i, c) in commits.iter().enumerate() {
        if !c.merge || c.parents.len() < 2 {
            continue;
        }
        let src = c.parents[1];
        let (sl, ml) = (lanes[commits[src].branch], lanes[c.branch]);
        let col = color(commits[src].branch);
        line(&mut cv, src as i64, sl, i as i64, sl, col);
        line(&mut cv, i as i64, sl, i as i64, ml, col);
        let (x, y) = pos(i as i64, sl);
        if let Some(cell) = cv.at(x, y) {
            cell.round = true;
        }
    }
    // Branch names, commits, tags.
    for (bi, b) in branches.iter().enumerate() {
        let (x, y) = pos(0, lanes[bi]);
        if tb {
            cv.text(0.max(x - width(&b.name) / 2), 0, &b.name, Role::ClusterTitle, color(bi), true);
        } else {
            cv.text(0, y, &b.name, Role::ClusterTitle, color(bi), true);
        }
    }
    for (i, c) in commits.iter().enumerate() {
        let (x, y) = pos(i as i64, lanes[c.branch]);
        let g = if c.merge {
            "◉"
        } else if c.cherry {
            "◎"
        } else if c.kind == "HIGHLIGHT" {
            "■"
        } else if c.kind == "REVERSE" {
            "⊗"
        } else {
            "●"
        };
        cv.text(x, y, g, Role::Bar, color(c.branch), false);
        let label = if c.tag.is_empty() { &c.id } else { &c.tag };
        if label.is_empty() {
            continue;
        }
        let role = if c.tag.is_empty() { Role::Muted } else { Role::Accent };
        let lw = width(label);
        if tb {
            if cv.blank(x + 2, y, lw) {
                cv.text(x + 2, y, label, role, "", false);
            }
        } else {
            let lx = x - lw / 2;
            if cv.blank(lx - 1, y - 1, lw + 2) {
                cv.text(lx, y - 1, label, role, "", false);
            }
        }
    }
    let mut p = cv.picture();
    if dir == "BT" {
        flip_vertical(&mut p);
    }
    Ok((p, String::new()))
}

/// Turns a picture upside down, mirroring line glyphs so they still join.
pub(crate) fn flip_vertical(p: &mut Picture) {
    p.cells.reverse();
    for row in &mut p.cells {
        for c in row.iter_mut() {
            let f = match c.s.as_str() {
                "┌" => "└",
                "└" => "┌",
                "┐" => "┘",
                "┘" => "┐",
                "╭" => "╰",
                "╰" => "╭",
                "╮" => "╯",
                "╯" => "╮",
                "┬" => "┴",
                "┴" => "┬",
                "▼" => "▲",
                "▲" => "▼",
                "▽" => "△",
                "△" => "▽",
                _ => continue,
            };
            c.s = f.to_string();
        }
    }
}
