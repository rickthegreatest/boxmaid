//! Drawing the ```mermaid blocks of a Markdown document.

use crate::re::lazy_re;
use crate::{render, Options};

lazy_re!(MERMAID_FENCE, "(?ms)^([ \t]*)(`{3,}|~{3,})[ \t]*mermaid[^\n]*\n(.*?)^[ \t]*(`{3,}|~{3,})[ \t]*$");

/// Draws the ```mermaid blocks of a Markdown document, replacing each with
/// a ```text block holding the drawing, so any Markdown renderer (a pager,
/// a chat message) shows the diagram. Blocks that can't be drawn are left
/// as they are.
pub fn replace_mermaid(md: &str, opt: &Options) -> String {
    MERMAID_FENCE
        .replace_all(md, |m: &regex::Captures| {
            let block = &m[0];
            let (indent, fence, src, close) = (&m[1], &m[2], &m[3], &m[4]);
            if !close.starts_with(&fence[..1]) || close.len() < fence.len() {
                return block.to_string();
            }
            let Ok(p) = render(src, opt) else { return block.to_string() };
            let mut b = format!("{indent}{fence}text\n");
            if !p.title.is_empty() {
                b += &format!("{indent}{}\n\n", p.title);
            }
            for l in p.lines() {
                b += &format!("{indent}{l}\n");
            }
            b += &format!("{indent}{fence}");
            b
        })
        .into_owned()
}
