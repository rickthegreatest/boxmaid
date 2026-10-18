//! Fenced-block transformation, without interpreting the rest of Markdown.

use crate::{render, Options};

struct Fence<'a> {
    indent: &'a str,
    marker: &'a str,
    info: &'a str,
}

fn parse_fence(line: &str) -> Option<Fence<'_>> {
    let line = line.trim_end_matches(['\r', '\n']);
    let rest = line.trim_start_matches([' ', '\t']);
    let first = *rest.as_bytes().first()?;
    if first != b'`' && first != b'~' {
        return None;
    }
    let n = rest.bytes().take_while(|&b| b == first).count();
    if n < 3 || first == b'`' && rest[n..].contains('`') {
        return None;
    }
    Some(Fence { indent: &line[..line.len() - rest.len()], marker: &rest[..n], info: rest[n..].trim() })
}

/// Replaces fenced mermaid blocks with fenced text drawings.
/// Preserves other fenced blocks, unsupported diagrams, unclosed fences,
/// and line endings. This is not a full Markdown parser: blockquote-prefixed
/// fences, for example, are left alone.
pub fn replace_mermaid(md: &str, opt: &Options) -> String {
    let lines: Vec<&str> = md.split_inclusive('\n').collect();
    let mut out = String::new();
    let mut i = 0;
    while i < lines.len() {
        let Some(open) = parse_fence(lines[i]) else {
            out.push_str(lines[i]);
            i += 1;
            continue;
        };
        let end = (i + 1..lines.len()).find(|&j| {
            parse_fence(lines[j])
                .is_some_and(|c| c.info.is_empty() && c.marker.as_bytes()[0] == open.marker.as_bytes()[0] && c.marker.len() >= open.marker.len())
        });
        let Some(end) = end else {
            out.push_str(&lines[i..].concat());
            break;
        };
        let pic = if open.info.split_whitespace().next() == Some("mermaid") {
            let src: String = lines[i + 1..end].iter().map(|l| l.strip_prefix(open.indent).unwrap_or(l)).collect();
            render(&src, opt).ok()
        } else {
            None
        };
        if let Some(pic) = pic {
            let eol = if lines[i].ends_with("\r\n") { "\r\n" } else { "\n" };
            out += &format!("{}{}text{eol}", open.indent, open.marker);
            if !pic.title.is_empty() {
                out += &format!("{}{title}{eol}{eol}", open.indent, title = pic.title);
            }
            for line in pic.lines() {
                out += &format!("{}{line}{eol}", open.indent);
            }
            out.push_str(lines[end]);
        } else {
            out.push_str(&lines[i..=end].concat());
        }
        i = end + 1;
    }
    out
}
