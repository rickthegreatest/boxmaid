//! A full-screen viewer: cargo run --example ratatui_viewer --features ratatui -- diagram.mmd
//!
//! Arrow keys (or hjkl) pan, q quits. The diagram is drawn to fit the
//! terminal's width and redrawn when the terminal is resized.

use ratatui::crossterm::event::{self, Event, KeyCode, KeyEventKind, KeyModifiers};
use ratatui::layout::{Constraint, Layout};
use ratatui::text::Line;
use ratatui::widgets::Paragraph;

use boxmaid::{render, Diagram, Options, Picture, Theme};

fn main() -> std::io::Result<()> {
    let Some(path) = std::env::args().nth(1) else {
        eprintln!("usage: ratatui_viewer diagram.mmd");
        std::process::exit(2);
    };
    let src = std::fs::read_to_string(&path)?;
    let mut terminal = ratatui::init();
    let (mut x, mut y) = (0usize, 0usize);
    let mut page_height = 1;
    let mut drawn: Option<(u16, Result<Picture, boxmaid::Error>)> = None;
    let result = loop {
        let r = terminal.draw(|f| {
            let [body, help] = Layout::vertical([Constraint::Fill(1), Constraint::Length(2)]).areas(f.area());
            page_height = (body.height as usize).saturating_sub(1).max(1);
            if drawn.as_ref().is_none_or(|(w, _)| *w != body.width) {
                drawn = Some((body.width, render(&src, &Options { width: body.width as usize, ascii: false })));
            }
            match &drawn.as_ref().unwrap().1 {
                Ok(p) => {
                    x = x.min(p.w.saturating_sub(body.width as usize));
                    y = y.min(p.h.saturating_sub(body.height as usize));
                    f.render_widget(Diagram::new(p).theme(Theme::default()).scroll(x, y), body);
                }
                Err(e) => f.render_widget(Line::from(format!("{path}: {e}")), body),
            }
            f.render_widget(
                Paragraph::new(vec![
                    Line::from(format!("{path} · column {}, row {}", x + 1, y + 1)),
                    Line::from("↑↓/hjkl pan · PgUp/PgDn page · Home/End · q quit"),
                ]),
                help,
            );
        });
        if let Err(e) = r {
            break Err(e);
        }
        match event::read() {
            Ok(Event::Key(k)) if k.kind == KeyEventKind::Press && k.code == KeyCode::Char('c') && k.modifiers.contains(KeyModifiers::CONTROL) => break Ok(()),
            Ok(Event::Key(k)) if k.kind == KeyEventKind::Press => match k.code {
                KeyCode::Char('q') | KeyCode::Esc => break Ok(()),
                KeyCode::Left | KeyCode::Char('h') => x = x.saturating_sub(4),
                KeyCode::Right | KeyCode::Char('l') => x = x.saturating_add(4),
                KeyCode::Up | KeyCode::Char('k') => y = y.saturating_sub(1),
                KeyCode::Down | KeyCode::Char('j') => y = y.saturating_add(1),
                KeyCode::PageUp => y = y.saturating_sub(page_height),
                KeyCode::PageDown | KeyCode::Char(' ') => y = y.saturating_add(page_height),
                KeyCode::Home | KeyCode::Char('g') => {
                    x = 0;
                    y = 0;
                }
                KeyCode::End | KeyCode::Char('G') => y = usize::MAX,
                _ => {}
            },
            Ok(_) => {}
            Err(e) => break Err(e),
        }
    };
    ratatui::restore();
    result
}
