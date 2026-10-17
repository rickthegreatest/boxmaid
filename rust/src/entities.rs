//! HTML entities in labels, and number formatting: fixed tables shared with
//! the Go implementation so both decode and print identically.

fn named(name: &str) -> Option<&'static str> {
    Some(match name {
        "amp" => "&",
        "lt" => "<",
        "gt" => ">",
        "quot" => "\"",
        "apos" => "'",
        "nbsp" => " ",
        "copy" => "©",
        "reg" => "®",
        "trade" => "™",
        "hellip" => "…",
        "mdash" => "—",
        "ndash" => "–",
        "laquo" => "«",
        "raquo" => "»",
        "lsquo" => "‘",
        "rsquo" => "’",
        "ldquo" => "“",
        "rdquo" => "”",
        "larr" => "←",
        "rarr" => "→",
        "uarr" => "↑",
        "darr" => "↓",
        "harr" => "↔",
        "lArr" => "⇐",
        "rArr" => "⇒",
        "times" => "×",
        "divide" => "÷",
        "deg" => "°",
        "plusmn" => "±",
        "middot" => "·",
        "bull" => "•",
        "euro" => "€",
        "pound" => "£",
        "yen" => "¥",
        "cent" => "¢",
        "sect" => "§",
        "para" => "¶",
        "le" => "≤",
        "ge" => "≥",
        "ne" => "≠",
        "infin" => "∞",
        "check" => "✓",
        "hearts" => "♥",
        "num" => "#",
        "colon" => ":",
        "semi" => ";",
        "lpar" => "(",
        "rpar" => ")",
        "lsqb" => "[",
        "rsqb" => "]",
        "lcub" => "{",
        "rcub" => "}",
        "vert" => "|",
        "excl" => "!",
        "quest" => "?",
        "percnt" => "%",
        "plus" => "+",
        "equals" => "=",
        "commat" => "@",
        "dollar" => "$",
        "ast" => "*",
        "sol" => "/",
        "bsol" => "\\",
        _ => return None,
    })
}

/// The text for an entity's name ("amp", "#35", "#x23").
pub(crate) fn decode_entity(name: &str) -> Option<String> {
    let (digits, radix) = if let Some(h) = name.strip_prefix("#x").or_else(|| name.strip_prefix("#X")) {
        (h, 16)
    } else if let Some(d) = name.strip_prefix('#') {
        (d, 10)
    } else {
        return named(name).map(str::to_string);
    };
    let n = u32::from_str_radix(digits, radix).ok()?;
    if n == 0 || n > 0x10FFFF {
        return None;
    }
    char::from_u32(n).map(|c| c.to_string())
}

/// Decodes &name; entities.
pub(crate) fn unescape_html(s: &str) -> String {
    if !s.contains('&') {
        return s.to_string();
    }
    let mut out = String::with_capacity(s.len());
    let mut rest = s;
    while let Some(i) = rest.find('&') {
        out.push_str(&rest[..i]);
        rest = &rest[i..];
        if let Some(j) = rest.find(';') {
            if j > 1 && j <= 12 {
                if let Some(d) = decode_entity(&rest[1..j]) {
                    out.push_str(&d);
                    rest = &rest[j + 1..];
                    continue;
                }
            }
        }
        out.push('&');
        rest = &rest[1..];
    }
    out.push_str(rest);
    out
}

/// A number written the same way as the Go implementation: whole numbers
/// without decimals, others with up to two.
pub(crate) fn format_number(v: f64) -> String {
    if v == (v as i64) as f64 && v < 1e15 && v > -1e15 {
        return (v as i64).to_string();
    }
    let s = format!("{:.2}", v);
    let s = s.trim_end_matches('0');
    s.trim_end_matches('.').to_string()
}
