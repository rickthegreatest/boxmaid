//! boxmaid draws [Mermaid](https://mermaid.js.org) diagrams for the
//! terminal: boxes, lines and charts from Unicode box drawing (or plain
//! ASCII), laid out so edges don't run through nodes or each other, and
//! sized to fit the width you have.
//!
//! ```
//! let pic = boxmaid::render("flowchart LR\n  A --> B", &boxmaid::Options::default()).unwrap();
//! println!("{pic}");                                   // plain text
//! println!("{}", pic.ansi(&boxmaid::Theme::default())); // coloured
//! ```
//!
//! [`render`] returns a [`Picture`]: a grid of cells, each with a [`Role`]
//! (node border, edge, label…) so a TUI can colour it with its own theme.
//! With the `ratatui` feature, a picture is also a ratatui widget.
//!
//! The Go module `github.com/boxmaid/boxmaid` produces the same output for
//! the same input: both are checked against one shared set of golden files.

#![forbid(unsafe_code)]
#![warn(missing_docs)]

mod ansi;
mod asciitext;
mod block;
mod c4;
mod canvas;
mod charts;
mod class;
mod dates;
mod draw;
mod entities;
mod er;
mod flowchart;
mod flows;
mod gantt;
mod gitgraph;
mod graph;
mod journey;
mod kanban;
mod layout;
mod live;
mod markdown;
mod mindmap;
mod pie;
mod re;
mod requirement;
mod sequence;
mod state;
mod text;
mod timeline;
#[cfg(feature = "ratatui")]
mod widget;
mod width;

use std::collections::BTreeMap;
use std::fmt;
use std::sync::LazyLock;

pub use ansi::{Style, Theme};
pub use live::live_url;
pub use markdown::replace_mermaid;
#[cfg(feature = "ratatui")]
pub use widget::Diagram;
pub use width::{rune_width, string_width};

/// What shapes the drawing.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Options {
    /// The room available, in columns. Rendering picks the layout that
    /// fits: it wraps labels tighter and turns a wide left-to-right
    /// flowchart top to bottom. 0 means unlimited.
    pub width: usize,
    /// Draw with plain ASCII characters only.
    pub ascii: bool,
}

/// What a cell is part of, for colouring.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Hash, PartialOrd, Ord)]
#[allow(missing_docs)]
pub enum Role {
    #[default]
    None,
    /// The diagram's title.
    Title,
    /// A node's border.
    Node,
    /// The text inside a node.
    NodeText,
    /// A decision (rhombus) node's border.
    Decision,
    /// Start and end markers, fork bars.
    Start,
    /// A connecting line.
    Edge,
    /// An arrowhead.
    Arrow,
    /// Text on a line.
    EdgeLabel,
    /// A subgraph's, namespace's or boundary's frame.
    Cluster,
    /// Its title.
    ClusterTitle,
    /// A note's border and text.
    Note,
    /// A sequence participant's box.
    Actor,
    /// A sequence participant's lifeline.
    Lifeline,
    /// A sequence loop/alt/opt frame.
    Block,
    /// Its label.
    BlockTitle,
    /// A bar or point of a chart.
    Bar,
    /// Secondary text (descriptions, values, axes).
    Muted,
    /// Highlighted text (active tasks, critical paths).
    Accent,
    /// Warnings (critical tasks, low scores).
    Warn,
    /// Good things (done tasks, high scores).
    Good,
}

/// One character cell of a [`Picture`]. A wide character occupies its cell
/// and the next, whose `s` is empty.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Cell {
    /// The character drawn here ("" for the right half of a wide one).
    pub s: String,
    /// What the cell is part of.
    pub role: Role,
    /// "#rrggbb" from the diagram's own styling (classDef, style), or "".
    pub color: String,
    /// Whether the text is bold.
    pub bold: bool,
    pub(crate) wide: bool,
}

impl Cell {
    /// Whether this is the right half of a wide character.
    pub fn is_continuation(&self) -> bool {
        self.wide
    }
}

/// A drawn diagram.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Picture {
    /// flowchart, sequence, class, state, er, gantt, …
    pub kind: String,
    /// The title from the source, if any.
    pub title: String,
    /// Width in cells.
    pub w: usize,
    /// Height in cells.
    pub h: usize,
    /// The cells, row by row.
    pub cells: Vec<Vec<Cell>>,
}

impl Picture {
    /// The picture as plain text lines, without trailing spaces.
    pub fn lines(&self) -> Vec<String> {
        self.cells
            .iter()
            .map(|row| {
                let s: String = row.iter().filter(|c| !c.wide).map(|c| c.s.as_str()).collect();
                s.trim_end_matches(' ').to_string()
            })
            .collect()
    }
}

impl fmt::Display for Picture {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.lines().join("\n"))
    }
}

/// Why a diagram couldn't be drawn.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Error {
    /// The diagram is empty.
    Empty,
    /// This kind of diagram can't be drawn in a terminal.
    Unsupported(String),
    /// The source couldn't be read as this kind of diagram.
    Parse(String),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Empty => f.write_str("the diagram is empty"),
            Error::Unsupported(k) => write!(f, "{k} diagrams can't be drawn in a terminal"),
            Error::Parse(m) => f.write_str(m),
        }
    }
}

impl std::error::Error for Error {}

/// What a renderer gives back: the picture and a title found in the source.
pub(crate) type Drawn = Result<(Picture, String), Error>;

type Renderer = fn(body: &str, rest: &str, opt: &Options) -> Drawn;

static KINDS: LazyLock<BTreeMap<&'static str, (&'static str, Renderer)>> = LazyLock::new(|| {
    let mut m: BTreeMap<&'static str, (&'static str, Renderer)> = BTreeMap::new();
    let mut reg = |name: &'static str, r: Renderer, headers: &[&'static str]| {
        for h in headers {
            m.insert(h, (name, r));
        }
    };
    reg("flowchart", flowchart::render, &["flowchart", "graph", "flowchart-elk"]);
    reg("state", state::render, &["stateDiagram", "stateDiagram-v2"]);
    reg("c4", c4::render, &["C4Context", "C4Container", "C4Component", "C4Dynamic", "C4Deployment"]);
    reg("sequence", sequence::render, &["sequenceDiagram"]);
    reg("class", class::render, &["classDiagram", "classDiagram-v2"]);
    reg("er", er::render, &["erDiagram"]);
    reg("gantt", gantt::render, &["gantt"]);
    reg("journey", journey::render, &["journey"]);
    reg("timeline", timeline::render, &["timeline"]);
    reg("mindmap", mindmap::render, &["mindmap"]);
    reg("gitgraph", gitgraph::render, &["gitGraph"]);
    reg("quadrant", charts::render_quadrant, &["quadrantChart"]);
    reg("xychart", charts::render_xychart, &["xychart-beta", "xychart"]);
    reg("kanban", kanban::render_kanban, &["kanban"]);
    reg("packet", kanban::render_packet, &["packet-beta", "packet"]);
    reg("sankey", flows::render_sankey, &["sankey-beta", "sankey"]);
    reg("radar", flows::render_radar, &["radar-beta", "radar"]);
    reg("treemap", flows::render_treemap, &["treemap-beta", "treemap"]);
    reg("requirement", requirement::render_requirement, &["requirementDiagram"]);
    reg("architecture", requirement::render_architecture, &["architecture-beta", "architecture"]);
    reg("block", block::render, &["block-beta", "block"]);
    reg("pie", pie::render, &["pie"]);
    m
});

/// Parses and draws a diagram.
pub fn render(src: &str, opt: &Options) -> Result<Picture, Error> {
    let (body, title) = clean(src);
    let (header, rest) = detect(&body);
    if header.is_empty() {
        return Err(Error::Empty);
    }
    let Some(&(name, draw)) = KINDS.get(header.as_str()) else {
        return Err(Error::Unsupported(header));
    };
    let (mut p, t) = draw(&body, &rest, opt)?;
    p.kind = name.to_string();
    p.title = if title.is_empty() { t } else { title };
    Ok(p)
}

/// The diagram headers boxmaid draws ("flowchart", "sequenceDiagram", …),
/// sorted.
pub fn kinds() -> Vec<&'static str> {
    KINDS.keys().copied().collect()
}

/// Whether src is a kind of diagram boxmaid draws.
pub fn supported(src: &str) -> bool {
    KINDS.contains_key(kind(src).as_str())
}

/// The diagram's header as its source names it ("flowchart",
/// "sequenceDiagram", "gantt"…), or "" for an empty one.
pub fn kind(src: &str) -> String {
    let (body, _) = clean(src);
    detect(&body).0
}

re::lazy_re!(FRONTMATTER, r"(?s)\A\s*---\s*\n(.*?)\n\s*---\s*\n");
re::lazy_re!(DIRECTIVE, r"(?s)%%\{.*?\}%%");
re::lazy_re!(FM_TITLE, r"(?m)^\s*title\s*:\s*(.+?)\s*$");

/// Drops front matter, %%{init}%% directives and %% comments, and returns
/// the front matter's title if it has one.
pub fn clean(src: &str) -> (String, String) {
    let mut s = src.replace("\r\n", "\n").replace('\t', "    ");
    let mut title = String::new();
    if let Some(c) = FRONTMATTER.captures(&s) {
        if let Some(t) = FM_TITLE.captures(&c[1]) {
            title = t[1].trim_matches(|c| c == '"' || c == '\'').to_string();
        }
        let end = c.get(0).unwrap().end();
        s = s[end..].to_string();
    }
    let s = DIRECTIVE.replace_all(&s, "");
    let lines: Vec<&str> = s.split('\n').filter(|l| !l.trim().starts_with("%%")).collect();
    (lines.join("\n"), title)
}

/// The diagram's header (its first word) and the text after the header
/// line.
pub(crate) fn detect(body: &str) -> (String, String) {
    let lines: Vec<&str> = body.split('\n').collect();
    for (i, l) in lines.iter().enumerate() {
        let t = l.trim();
        if t.is_empty() {
            continue;
        }
        let first = t.split_whitespace().next().unwrap_or("");
        let k = first.strip_suffix(';').unwrap_or(first);
        let k = k.strip_suffix(':').unwrap_or(k);
        return (k.to_string(), lines[i + 1..].join("\n"));
    }
    (String::new(), String::new())
}
