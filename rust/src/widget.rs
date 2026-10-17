//! A ratatui widget for pictures.

use ratatui::buffer::Buffer;
use ratatui::layout::Rect;
use ratatui::style::{Color, Modifier, Style as TuiStyle};
use ratatui::widgets::Widget;

use crate::{Picture, Theme};

/// A picture drawn with a theme, scrolled to an offset. A `&Picture` is a
/// widget too, drawn with the default theme from its top left corner.
///
/// ```ignore
/// let pic = boxmaid::render(src, &boxmaid::Options { width: area.width as usize, ..Default::default() })?;
/// frame.render_widget(boxmaid::Diagram::new(&pic).theme(boxmaid::Theme::nord()).scroll(0, top), area);
/// ```
#[derive(Clone, Debug)]
pub struct Diagram<'a> {
    picture: &'a Picture,
    theme: Theme,
    x: usize,
    y: usize,
}

impl<'a> Diagram<'a> {
    /// The picture with the default theme.
    pub fn new(picture: &'a Picture) -> Self {
        Diagram { picture, theme: Theme::default(), x: 0, y: 0 }
    }

    /// Draws with this theme.
    pub fn theme(mut self, t: Theme) -> Self {
        self.theme = t;
        self
    }

    /// Starts drawing at column x and row y of the picture.
    pub fn scroll(mut self, x: usize, y: usize) -> Self {
        self.x = x;
        self.y = y;
        self
    }
}

/// The ratatui colour of a theme colour: an ANSI number or "#rrggbb".
fn color(c: &str, true_color: bool) -> Option<Color> {
    if let Some(h) = c.strip_prefix('#') {
        if !true_color || h.len() != 6 {
            return None;
        }
        let p = |i: usize| u8::from_str_radix(h.get(i..i + 2)?, 16).ok();
        return Some(Color::Rgb(p(0)?, p(2)?, p(4)?));
    }
    let n: u8 = c.parse().ok()?;
    Some(match n {
        0 => Color::Black,
        1 => Color::Red,
        2 => Color::Green,
        3 => Color::Yellow,
        4 => Color::Blue,
        5 => Color::Magenta,
        6 => Color::Cyan,
        7 => Color::Gray,
        8 => Color::DarkGray,
        9 => Color::LightRed,
        10 => Color::LightGreen,
        11 => Color::LightYellow,
        12 => Color::LightBlue,
        13 => Color::LightMagenta,
        14 => Color::LightCyan,
        15 => Color::White,
        n => Color::Indexed(n),
    })
}

impl Widget for Diagram<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        let t = &self.theme;
        for (dy, row) in self.picture.cells.iter().skip(self.y).take(area.height as usize).enumerate() {
            let y = area.y + dy as u16;
            let right = area.x as usize + area.width as usize;
            for (dx, c) in row.iter().skip(self.x).take(area.width as usize).enumerate() {
                let x = area.x + dx as u16;
                let Some(cell) = buf.cell_mut((x, y)) else { continue };
                if c.wide {
                    // The right half of a wide character: its left half
                    // covers it (or, scrolled off, leaves a blank).
                    if dx == 0 {
                        cell.reset();
                    }
                    continue;
                }
                let mut st = t.style(c.role);
                if !c.color.is_empty() && t.true_color {
                    st.color = c.color.clone();
                }
                let mut s = TuiStyle::default();
                if let Some(fg) = color(&st.color, t.true_color) {
                    s = s.fg(fg);
                }
                if st.bold || c.bold {
                    s = s.add_modifier(Modifier::BOLD);
                }
                let wide_cut = x as usize + 1 >= right && row.get(self.x + dx + 1).is_some_and(|n| n.wide);
                cell.set_symbol(if wide_cut || c.s.is_empty() { " " } else { &c.s });
                cell.set_style(s);
            }
        }
    }
}

impl Widget for &Picture {
    fn render(self, area: Rect, buf: &mut Buffer) {
        Diagram::new(self).render(area, buf);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn draws_into_a_buffer() {
        let cell = |s: &str| crate::Cell { s: s.to_string(), ..Default::default() };
        let p = Picture { w: 3, h: 1, cells: vec![vec![cell("A"), cell("界"), crate::Cell { wide: true, ..Default::default() }]], ..Default::default() };
        let area = Rect::new(0, 0, 40, 10);
        let mut buf = Buffer::empty(area);
        (&p).render(area, &mut buf);
        let text: String = (0..area.height).flat_map(|y| (0..area.width).map(move |x| (x, y))).map(|(x, y)| buf[(x, y)].symbol().to_string()).collect();
        assert!(text.starts_with("A界"));
    }
}
