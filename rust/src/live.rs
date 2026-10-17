//! Links to the Mermaid Live Editor.

/// A link that opens the diagram in the Mermaid Live Editor's viewer. The
/// diagram travels in the URL fragment, which browsers don't send to the
/// server.
///
/// The state is zlib data like the editor's own (pako) links, written with
/// stored deflate blocks so the crate needs no compression library; the
/// editor reads it the same. The Go module compresses, so its links are
/// shorter but open the same diagram.
pub fn live_url(src: &str) -> String {
    let state = format!(r#"{{"code":{},"mermaid":{}}}"#, json_string(src), json_string(r#"{"theme":"default"}"#));
    format!("https://mermaid.live/view#pako:{}", base64_url(&zlib_stored(state.as_bytes())))
}

/// s as a JSON string, escaped the way Go's encoding/json does it.
fn json_string(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '\u{8}' => out.push_str("\\b"),
            '\u{c}' => out.push_str("\\f"),
            '<' | '>' | '&' | '\u{2028}' | '\u{2029}' => out.push_str(&format!("\\u{:04x}", c as u32)),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

/// data as a zlib stream of stored (uncompressed) deflate blocks.
fn zlib_stored(data: &[u8]) -> Vec<u8> {
    let mut out = vec![0x78, 0x01];
    let mut chunks = data.chunks(0xffff).peekable();
    if chunks.peek().is_none() {
        out.extend_from_slice(&[1, 0, 0, 0xff, 0xff]);
    }
    while let Some(c) = chunks.next() {
        out.push(u8::from(chunks.peek().is_none()));
        let n = c.len() as u16;
        out.extend_from_slice(&n.to_le_bytes());
        out.extend_from_slice(&(!n).to_le_bytes());
        out.extend_from_slice(c);
    }
    let (mut a, mut b) = (1u32, 0u32);
    for &x in data {
        a = (a + u32::from(x)) % 65521;
        b = (b + a) % 65521;
    }
    out.extend_from_slice(&((b << 16) | a).to_be_bytes());
    out
}

/// Unpadded URL-safe base64.
fn base64_url(data: &[u8]) -> String {
    const ABC: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    let mut out = String::with_capacity(data.len() * 4 / 3 + 3);
    for c in data.chunks(3) {
        let v = (u32::from(c[0]) << 16) | (u32::from(*c.get(1).unwrap_or(&0)) << 8) | u32::from(*c.get(2).unwrap_or(&0));
        for i in 0..=c.len() {
            out.push(ABC[(v >> (18 - 6 * i)) as usize & 63] as char);
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Inflates stored blocks back, checking the framing.
    fn unstore(z: &[u8]) -> Vec<u8> {
        assert_eq!((u32::from(z[0]) * 256 + u32::from(z[1])) % 31, 0);
        let (mut i, mut out) = (2, vec![]);
        loop {
            let last = z[i] & 1 == 1;
            let n = u16::from_le_bytes([z[i + 1], z[i + 2]]) as usize;
            assert_eq!(!u16::from_le_bytes([z[i + 1], z[i + 2]]), u16::from_le_bytes([z[i + 3], z[i + 4]]));
            out.extend_from_slice(&z[i + 5..i + 5 + n]);
            i += 5 + n;
            if last {
                break;
            }
        }
        assert_eq!(i + 4, z.len());
        out
    }

    #[test]
    fn stored_round_trip() {
        for n in [0, 1, 70000, 0xffff, 0x1_0000 * 2] {
            let data: Vec<u8> = (0..n).map(|i| (i * 7) as u8).collect();
            assert_eq!(unstore(&zlib_stored(&data)), data);
        }
    }

    #[test]
    fn json_like_go() {
        assert_eq!(json_string("a\"<b>\n\u{1}é"), "\"a\\\"\\u003cb\\u003e\\n\\u0001é\"");
    }

    #[test]
    fn base64() {
        assert_eq!(base64_url(b"\xfb\xff"), "-_8");
        assert_eq!(base64_url(b"abc"), "YWJj");
    }
}
