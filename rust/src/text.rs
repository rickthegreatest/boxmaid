//! Labels and wrapping.

use crate::entities::{decode_entity, unescape_html};
use crate::re::lazy_re;
use crate::width::{graphemes, string_width};

lazy_re!(BR_TAG, r"(?i)<br\s*/?>");
lazy_re!(ANY_TAG, r"</?[a-zA-Z][^<>]*>");
lazy_re!(ENTITY, r"#([a-zA-Z]+|\d+);");
lazy_re!(FA_ICON, r"\b(fa[bsrl]?|fa-solid|fa-regular):fa-[a-z0-9-]+\s*");
lazy_re!(MD_STRONG, r"\*\*(.+?)\*\*|__(.+?)__");
lazy_re!(MD_EMPH, r"(^|[^*\w])\*([^*\s][^*]*?)\*|(^|[^_\w])_([^_\s][^_]*?)_");
lazy_re!(SPACE_RUNS, r"[ \t]+");

/// Turns a Mermaid label into plain lines: <br> and "\n" break lines,
/// Mermaid's #quot; style entities and HTML entities decode, tags, icons
/// and Markdown emphasis drop away.
pub(crate) fn label(s: &str) -> Vec<String> {
    let mut s = s.trim().to_string();
    let b = s.as_bytes();
    if b.len() >= 2 && ((b[0] == b'"' && b[b.len() - 1] == b'"') || (b[0] == b'`' && b[b.len() - 1] == b'`')) {
        s = s[1..s.len() - 1].trim().trim_matches('`').to_string();
    }
    s = BR_TAG.replace_all(&s, "\n").into_owned();
    s = s.replace("\\n", "\n");
    s = ANY_TAG.replace_all(&s, "").into_owned();
    s = ENTITY
        .replace_all(&s, |c: &regex::Captures| {
            let e = &c[0];
            let mut name = e[1..e.len() - 1].to_string();
            if name.as_bytes().first().is_some_and(|b| b.is_ascii_digit()) {
                name = format!("#{name}");
            }
            decode_entity(&name).unwrap_or_else(|| e.to_string())
        })
        .into_owned();
    s = unescape_html(&s);
    s = FA_ICON.replace_all(&s, "").into_owned();
    s = MD_STRONG.replace_all(&s, "${1}${2}").into_owned();
    s = MD_EMPH.replace_all(&s, "${1}${2}${3}${4}").into_owned();
    let mut out: Vec<String> = s.split('\n').map(|l| SPACE_RUNS.replace_all(l, " ").trim().to_string()).collect();
    while out.len() > 1 && out.last().is_some_and(|l| l.is_empty()) {
        out.pop();
    }
    out
}

/// Breaks lines longer than max columns at spaces (or anywhere, for words
/// longer than max).
pub(crate) fn wrap(lines: &[String], max: i64) -> Vec<String> {
    if max <= 0 {
        return lines.to_vec();
    }
    let max_w = max as usize;
    let mut out = Vec::new();
    for l in lines {
        if string_width(l) <= max_w {
            out.push(l.clone());
            continue;
        }
        let mut cur = String::new();
        for word in l.split_whitespace() {
            // Match the canvas's visible text before using byte offsets to split it.
            let mut w: String = graphemes(word).iter().map(|g| g.0).collect();
            while string_width(&w) > max_w {
                let head = break_word(&w, max_w);
                if !cur.is_empty() {
                    out.push(std::mem::take(&mut cur));
                }
                w = w[head.len()..].to_string();
                out.push(head);
            }
            if cur.is_empty() {
                cur = w;
            } else if string_width(&cur) + 1 + string_width(&w) <= max_w {
                cur.push(' ');
                cur.push_str(&w);
            } else {
                out.push(std::mem::replace(&mut cur, w));
            }
        }
        out.push(cur);
    }
    out
}

pub(crate) fn width(s: &str) -> i64 {
    string_width(s) as i64
}

pub(crate) fn max_width<S: AsRef<str>>(lines: &[S]) -> i64 {
    lines.iter().map(|l| width(l.as_ref())).max().unwrap_or(0)
}

/// The first piece of a word too long for a line: up to a natural break
/// (after _ - / . , or between a lower and an upper case letter) when there
/// is one in the second half, else exactly max columns.
fn break_word(w: &str, max: usize) -> String {
    let gs = graphemes(w);
    if gs.is_empty() {
        return w.to_string();
    }
    let (mut n, mut used) = (0, 0);
    while n < gs.len() && used + gs[n].1 <= max {
        used += gs[n].1;
        n += 1;
    }
    if n == 0 {
        // A wide grapheme in one column: take it anyway to make progress.
        return gs[0].0.to_string();
    }
    for i in (n / 2..n).rev() {
        let r = gs[i].0.chars().next().unwrap_or('\0');
        let next = gs.get(i + 1).and_then(|g| g.0.chars().next()).unwrap_or('\0');
        if "_-/.,:;|".contains(r) || r.is_lowercase() && next.is_uppercase() {
            return gs[..=i].iter().map(|g| g.0).collect();
        }
    }
    gs[..n].iter().map(|g| g.0).collect()
}

/// Splits at sep outside double quotes.
pub(crate) fn split_top_level(s: &str, sep: char) -> Vec<String> {
    let mut out = Vec::new();
    let mut quoted = false;
    let mut start = 0;
    for (i, r) in s.char_indices() {
        if r == '"' {
            quoted = !quoted;
        }
        if r == sep && !quoted {
            out.push(s[start..i].to_string());
            start = i + r.len_utf8();
        }
    }
    out.push(s[start..].to_string());
    out
}

pub(crate) fn or_str(a: &str, b: &str) -> String {
    if a.is_empty() {
        b.to_string()
    } else {
        a.to_string()
    }
}

pub(crate) fn pad_right(s: &str, w: i64) -> String {
    let n = w - width(s);
    if n > 0 {
        format!("{s}{}", " ".repeat(n as usize))
    } else {
        s.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wrapping_preserves_visible_graphemes() {
        for word in [format!("\u{200b}{}", "界".repeat(30)), format!("{}-\u{0301}{}", "x".repeat(15), "y".repeat(20)), "👩‍💻".repeat(20)] {
            let lines = wrap(std::slice::from_ref(&word), 28);
            assert!(lines.iter().all(|l| string_width(l) <= 28), "{lines:?}");
            let visible: String = graphemes(&word).iter().map(|g| g.0).collect();
            assert_eq!(lines.concat(), visible);
        }
    }
}
