//! A full-screen viewer: cargo run --example ratatui_viewer --features ratatui -- diagram.mmd
//!
//! Arrow keys (or hjkl) pan, q quits. The diagram is drawn to fit the
//! terminal's width and redrawn when the terminal is resized.

use ratatui::crossterm::event::{self, Event, KeyCode, KeyEventKind};
use ratatui::layout::{Constraint, Layout};
use ratatui::text::Line;

use boxmaid::{render, Diagram, Options, Picture, Theme};

fn main() -> std::io::Result<()> {
    let Some(path) = std::env::args().nth(1) else {
        eprintln!("usage: ratatui_viewer diagram.mmd");
        std::process::exit(2);
    };
    let src = std::fs::read_to_string(&path)?;
    let mut terminal = ratatui::init();
    let (mut x, mut y) = (0usize, 0usize);
    let mut drawn: Option<(u16, Result<Picture, boxmaid::Error>)> = None;
    let result = loop {
        let r = terminal.draw(|f| {
            let [body, help] = Layout::vertical([Constraint::Fill(1), Constraint::Length(1)]).areas(f.area());
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
            f.render_widget(Line::from("←→↑↓ pan · q quit"), help);
        });
        if let Err(e) = r {
            break Err(e);
        }
        match event::read() {
            Ok(Event::Key(k)) if k.kind == KeyEventKind::Press => match k.code {
                KeyCode::Char('q') | KeyCode::Esc => break Ok(()),
                KeyCode::Left | KeyCode::Char('h') => x = x.saturating_sub(4),
                KeyCode::Right | KeyCode::Char('l') => x += 4,
                KeyCode::Up | KeyCode::Char('k') => y = y.saturating_sub(1),
                KeyCode::Down | KeyCode::Char('j') => y += 1,
                _ => {}
            },
            Ok(_) => {}
            Err(e) => break Err(e),
        }
    };
    ratatui::restore();
    result
}
