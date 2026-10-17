//! Regular expressions with Go's semantics. Go's `\w`, `\s`, `\d` and `\b`
//! are ASCII-only while the regex crate's are Unicode-aware, so patterns
//! (written once, shared with the Go implementation) are rewritten to
//! explicit ASCII classes before compiling. That keeps both libraries
//! matching exactly the same text.

use regex::Regex;

/// Compiles a Go-syntax pattern with Go's ASCII meanings for the Perl
/// classes.
pub(crate) fn re(pattern: &str) -> Regex {
    let mut out = String::with_capacity(pattern.len() + 16);
    let mut in_class = false;
    let mut chars = pattern.chars().peekable();
    while let Some(c) = chars.next() {
        match c {
            '\\' => {
                let Some(n) = chars.next() else {
                    out.push('\\');
                    break;
                };
                let rep = match (n, in_class) {
                    ('w', false) => Some("[0-9A-Za-z_]"),
                    ('w', true) => Some("0-9A-Za-z_"),
                    ('W', false) => Some("[^0-9A-Za-z_]"),
                    ('s', false) => Some("[\\t\\n\\x0C\\r ]"),
                    ('s', true) => Some("\\t\\n\\x0C\\r "),
                    ('S', false) => Some("[^\\t\\n\\x0C\\r ]"),
                    ('d', false) => Some("[0-9]"),
                    ('d', true) => Some("0-9"),
                    ('D', false) => Some("[^0-9]"),
                    ('b', false) => Some("(?-u:\\b)"),
                    ('B', false) => Some("(?-u:\\B)"),
                    ('W' | 'S' | 'D', true) => panic!("negated class escape inside a class: {pattern}"),
                    _ => None,
                };
                match rep {
                    Some(r) => out.push_str(r),
                    None => {
                        out.push('\\');
                        out.push(n);
                    }
                }
            }
            '[' if !in_class => {
                in_class = true;
                out.push('[');
                // A leading ] or ^] is literal.
                if chars.peek() == Some(&'^') {
                    out.push(chars.next().unwrap());
                }
                if chars.peek() == Some(&']') {
                    out.push_str("\\]");
                    chars.next();
                }
            }
            ']' if in_class => {
                in_class = false;
                out.push(']');
            }
            _ => out.push(c),
        }
    }
    Regex::new(&out).unwrap_or_else(|e| panic!("bad pattern {pattern}: {e}"))
}

/// A lazily compiled pattern.
macro_rules! lazy_re {
    ($name:ident, $pat:expr) => {
        static $name: std::sync::LazyLock<regex::Regex> = std::sync::LazyLock::new(|| crate::re::re($pat));
    };
}
pub(crate) use lazy_re;

/// The text of capture group i, or "" when it didn't take part (Go's
/// FindStringSubmatch gives "" too).
pub(crate) fn group<'h>(c: &regex::Captures<'h>, i: usize) -> &'h str {
    c.get(i).map_or("", |m| m.as_str())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ascii_classes() {
        assert!(!re(r"^\w+$").is_match("Ünï"));
        assert!(re(r"^[\w-]+$").is_match("a_b-9"));
        assert!(!re(r"^\s$").is_match("\u{a0}"));
        assert!(re(r"\bfa:").is_match("x fa:fa-car"));
        assert!(re(r"[^)\s>]+").is_match("abc"));
    }
}
