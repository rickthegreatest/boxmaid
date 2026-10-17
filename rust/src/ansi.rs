//! Colouring a picture for a terminal.

use std::collections::HashMap;

use crate::{Picture, Role};

/// How cells of one role look: a colour (an ANSI colour number 0–255, or
/// "#rrggbb") and bold.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Style {
    /// An ANSI colour number ("4", "208") or "#rrggbb"; "" for none.
    pub color: String,
    /// Whether the text is bold.
    pub bold: bool,
}

/// Maps roles to styles. `true_color` lets the diagram's own colours
/// (classDef, style) through; without it they're ignored, so a theme made
/// of the 16 basic colours works in every terminal.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Theme {
    /// The theme's name.
    pub name: String,
    /// The style of each role; roles not listed are drawn plain.
    pub styles: HashMap<Role, Style>,
    /// Whether "#rrggbb" colours are used.
    pub true_color: bool,
}

fn theme(name: &str, true_color: bool, styles: &[(Role, &str, bool)]) -> Theme {
    Theme { name: name.to_string(), true_color, styles: styles.iter().map(|&(r, c, b)| (r, Style { color: c.to_string(), bold: b })).collect() }
}

/// The default theme uses the terminal's own 16 colours, so it follows the
/// user's colour scheme, dark or light.
impl Default for Theme {
    fn default() -> Theme {
        use Role::*;
        theme(
            "default",
            false,
            &[
                (Title, "", true),
                (Node, "4", false),
                (NodeText, "", true),
                (Decision, "3", false),
                (Start, "", true),
                (Edge, "8", false),
                (Arrow, "", true),
                (EdgeLabel, "6", false),
                (Cluster, "8", false),
                (ClusterTitle, "5", true),
                (Note, "3", false),
                (Actor, "4", false),
                (Lifeline, "8", false),
                (Block, "8", false),
                (BlockTitle, "6", true),
                (Bar, "4", false),
                (Muted, "8", false),
                (Accent, "6", true),
                (Warn, "1", true),
                (Good, "2", false),
            ],
        )
    }
}

impl Theme {
    /// No colour at all, only bold for names and titles.
    pub fn mono() -> Theme {
        use Role::*;
        theme("mono", false, &[(Title, "", true), (NodeText, "", true), (ClusterTitle, "", true), (BlockTitle, "", true), (Arrow, "", true), (Warn, "", true)])
    }

    /// A true-colour theme after the Dracula editor theme.
    pub fn dracula() -> Theme {
        use Role::*;
        theme(
            "dracula",
            true,
            &[
                (Title, "#f8f8f2", true),
                (Node, "#bd93f9", false),
                (NodeText, "#f8f8f2", true),
                (Decision, "#ffb86c", false),
                (Start, "#f8f8f2", true),
                (Edge, "#6272a4", false),
                (Arrow, "#f8f8f2", true),
                (EdgeLabel, "#8be9fd", false),
                (Cluster, "#6272a4", false),
                (ClusterTitle, "#ff79c6", true),
                (Note, "#f1fa8c", false),
                (Actor, "#bd93f9", false),
                (Lifeline, "#6272a4", false),
                (Block, "#6272a4", false),
                (BlockTitle, "#8be9fd", true),
                (Bar, "#bd93f9", false),
                (Muted, "#6272a4", false),
                (Accent, "#8be9fd", true),
                (Warn, "#ff5555", true),
                (Good, "#50fa7b", false),
            ],
        )
    }

    /// A true-colour theme after the Nord editor theme.
    pub fn nord() -> Theme {
        use Role::*;
        theme(
            "nord",
            true,
            &[
                (Title, "#eceff4", true),
                (Node, "#81a1c1", false),
                (NodeText, "#eceff4", true),
                (Decision, "#ebcb8b", false),
                (Start, "#eceff4", true),
                (Edge, "#4c566a", false),
                (Arrow, "#d8dee9", true),
                (EdgeLabel, "#88c0d0", false),
                (Cluster, "#4c566a", false),
                (ClusterTitle, "#b48ead", true),
                (Note, "#ebcb8b", false),
                (Actor, "#81a1c1", false),
                (Lifeline, "#4c566a", false),
                (Block, "#4c566a", false),
                (BlockTitle, "#88c0d0", true),
                (Bar, "#5e81ac", false),
                (Muted, "#616e88", false),
                (Accent, "#88c0d0", true),
                (Warn, "#bf616a", true),
                (Good, "#a3be8c", false),
            ],
        )
    }

    /// The names of the built-in themes, sorted.
    pub fn names() -> Vec<&'static str> {
        vec!["default", "dracula", "mono", "nord"]
    }

    /// A built-in theme by name.
    pub fn by_name(name: &str) -> Option<Theme> {
        match name {
            "default" => Some(Theme::default()),
            "mono" => Some(Theme::mono()),
            "dracula" => Some(Theme::dracula()),
            "nord" => Some(Theme::nord()),
            _ => None,
        }
    }

    /// The style of cells with this role ("" colour, not bold when the
    /// theme doesn't list it).
    pub fn style(&self, r: Role) -> Style {
        self.styles.get(&r).cloned().unwrap_or_default()
    }
}

fn is_hex_color(c: &str) -> bool {
    c.len() == 7 && c.starts_with('#') && c[1..].bytes().all(|b| b.is_ascii_hexdigit())
}

/// The escape sequence that switches to st ("" for none).
fn sgr(st: &Style, true_color: bool) -> String {
    let mut codes: Vec<String> = vec![];
    if st.bold {
        codes.push("1".into());
    }
    let c = st.color.as_str();
    if is_hex_color(c) && true_color {
        let p = |i: usize| u8::from_str_radix(&c[i..i + 2], 16).unwrap_or(0);
        codes.push(format!("38;2;{};{};{}", p(1), p(3), p(5)));
    } else if !c.is_empty() && !c.starts_with('#') {
        if let Ok(n) = c.parse::<i64>() {
            codes.push(if n < 8 {
                (30 + n).to_string()
            } else if n < 16 {
                (90 + n - 8).to_string()
            } else {
                format!("38;5;{n}")
            });
        }
    }
    if codes.is_empty() {
        return String::new();
    }
    format!("\x1b[{}m", codes.join(";"))
}

impl Picture {
    /// The picture coloured with the theme, as text for a terminal.
    pub fn ansi(&self, t: &Theme) -> String {
        let mut lines = Vec::with_capacity(self.cells.len());
        for row in &self.cells {
            let mut b = String::new();
            let mut cur = String::new();
            let mut end = row.len();
            while end > 0 && (row[end - 1].s == " " || row[end - 1].s.is_empty()) && !row[end - 1].wide {
                end -= 1;
            }
            for c in &row[..end] {
                if c.wide {
                    continue;
                }
                let mut st = t.style(c.role);
                if !c.color.is_empty() && t.true_color {
                    st.color = c.color.clone();
                }
                if c.bold {
                    st.bold = true;
                }
                let code = if c.s != " " { sgr(&st, t.true_color) } else { String::new() };
                if code != cur {
                    if !cur.is_empty() {
                        b.push_str("\x1b[0m");
                    }
                    b.push_str(&code);
                    cur = code;
                }
                b.push_str(&c.s);
            }
            if !cur.is_empty() {
                b.push_str("\x1b[0m");
            }
            lines.push(b);
        }
        lines.join("\n")
    }
}
