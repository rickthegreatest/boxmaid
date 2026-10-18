//! Pie charts, drawn as horizontal bars, largest first.

use crate::canvas::Canvas;
use crate::entities::format_number;
use crate::re::lazy_re;
use crate::text::width;
use crate::width::truncate;
use crate::{Drawn, Error, Options, Role};

lazy_re!(PIE_SLICE, r#"^"([^"]*)"\s*:\s*([0-9.]+)\s*$"#);

pub(crate) fn render(body: &str, _rest: &str, opt: &Options) -> Drawn {
    let mut slices: Vec<(String, f64)> = vec![];
    let mut title = String::new();
    let mut show_data = false;
    for (i, raw) in body.split('\n').enumerate() {
        let mut line = raw.trim();
        if i == 0 || line.starts_with("pie") {
            line = line.strip_prefix("pie").unwrap_or(line).trim();
            if let Some(l) = line.strip_prefix("showData") {
                show_data = true;
                line = l.trim();
            }
        }
        if let Some(t) = line.strip_prefix("title") {
            title = t.trim().to_string();
            continue;
        }
        if let Some(m) = PIE_SLICE.captures(line) {
            if let Ok(v) = m[2].parse::<f64>() {
                if v > 0.0 && v.is_finite() {
                    slices.push((m[1].to_string(), v));
                }
            }
        }
    }
    if slices.is_empty() {
        return Err(Error::Empty);
    }
    // Largest first, keeping the order of equal slices.
    slices.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap());
    // Scale before summing so several finite values cannot overflow the total.
    let largest = slices[0].1;
    let total: f64 = slices.iter().map(|s| s.1 / largest).sum();
    let name_w = slices.iter().map(|s| width(&s.0).min(30)).max().unwrap_or(0).max(0);
    let mut bar_w = 40i64;
    if opt.width > 0 {
        bar_w = (opt.width as i64 - name_w - 20).clamp(10, 40);
    }
    let mut cv = Canvas::new(name_w + bar_w + 24, slices.len() as i64, opt.ascii);
    for (i, (name, v)) in slices.iter().enumerate() {
        let y = i as i64;
        let mut name = name.clone();
        if width(&name) > name_w {
            name = truncate(&name, (name_w - 1) as usize) + "…";
        }
        cv.text(0, y, &name, Role::NodeText, "", false);
        let frac = (v / largest) / total;
        let n = (frac * bar_w as f64 + 0.5) as i64;
        let bar = if opt.ascii { "#" } else { "█" }.repeat(n as usize);
        cv.text(name_w + 2, y, &bar, Role::Bar, "", false);
        let mut info = format!("{:.1}%", frac * 100.0);
        if show_data {
            info += &format!("  ({})", format_number(*v));
        }
        cv.text(name_w + 3 + n, y, &info, Role::Muted, "", false);
    }
    Ok((cv.picture(), title))
}
