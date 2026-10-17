//! boxmaid draws Mermaid diagrams in the terminal.
//!
//! ```text
//! boxmaid diagram.mmd           draw a diagram
//! boxmaid < diagram.mmd         from standard input
//! boxmaid -md README.md         draw every ```mermaid block of a Markdown file
//! boxmaid -live diagram.mmd     print a mermaid.live link instead
//! boxmaid -list                 the diagram types boxmaid draws
//! ```

use std::io::{IsTerminal, Read};
use std::process::{exit, Command, Stdio};

use boxmaid::{Options, Theme};

struct Flags {
    width: usize,
    ascii: bool,
    color: String,
    theme: String,
    md: bool,
    live: bool,
    list: bool,
    inputs: Vec<String>,
}

fn usage() -> ! {
    eprintln!(
        "usage: boxmaid [flags] [file …]   (reads standard input without files)
  -ascii
    \tplain ASCII only
  -color string
    \tcolour: auto, always or never (default \"auto\")
  -list
    \tlist the diagram types boxmaid draws
  -live
    \tprint a mermaid.live link instead of drawing
  -md
    \tthe input is Markdown: draw its ```mermaid blocks
  -theme string
    \tcolour theme: {} (default \"default\")
  -w int
    \twidth to fit in (default: the terminal's)",
        Theme::names().join(", ")
    );
    exit(2)
}

/// Reads flags the way Go's flag package does: -name, --name, -name=value
/// or -name value, up to the first argument that isn't a flag (or "--").
fn parse_flags() -> Flags {
    let mut f = Flags { width: 0, ascii: false, color: "auto".into(), theme: "default".into(), md: false, live: false, list: false, inputs: vec![] };
    let mut args = std::env::args().skip(1);
    while let Some(a) = args.next() {
        if a == "--" {
            break;
        }
        if a == "-" || !a.starts_with('-') {
            f.inputs.push(a);
            break;
        }
        let name = a.trim_start_matches('-');
        let (name, value) = match name.split_once('=') {
            Some((n, v)) => (n, Some(v.to_string())),
            None => (name, None),
        };
        let boolean = |v: Option<String>| match v.as_deref() {
            None | Some("1" | "t" | "T" | "true" | "TRUE" | "True") => true,
            Some("0" | "f" | "F" | "false" | "FALSE" | "False") => false,
            Some(v) => {
                eprintln!("invalid boolean value {v:?} for -{name}");
                usage()
            }
        };
        let mut string = |v: Option<String>| {
            v.or_else(|| args.next()).unwrap_or_else(|| {
                eprintln!("flag needs an argument: -{name}");
                usage()
            })
        };
        match name {
            "w" => {
                let v = string(value);
                f.width = v.parse().unwrap_or_else(|_| {
                    eprintln!("invalid value {v:?} for flag -w: parse error");
                    usage()
                })
            }
            "ascii" => f.ascii = boolean(value),
            "md" => f.md = boolean(value),
            "live" => f.live = boolean(value),
            "list" => f.list = boolean(value),
            "color" => f.color = string(value),
            "theme" => f.theme = string(value),
            "h" | "help" => usage(),
            _ => {
                eprintln!("flag provided but not defined: -{name}");
                usage()
            }
        }
    }
    f.inputs.extend(args);
    f
}

/// The terminal's width: $COLUMNS, else what stty reports, else 100.
fn terminal_width() -> usize {
    if let Some(n) = std::env::var("COLUMNS").ok().and_then(|c| c.trim().parse().ok()).filter(|&n| n > 0) {
        return n;
    }
    let tty = std::fs::File::open("/dev/tty").map(Stdio::from).unwrap_or_else(|_| Stdio::inherit());
    if let Ok(out) = Command::new("stty").arg("size").stdin(tty).stderr(Stdio::null()).output() {
        let s = String::from_utf8_lossy(&out.stdout);
        if let Some(n) = s.split_whitespace().nth(1).and_then(|c| c.parse().ok()).filter(|&n: &usize| n > 0) {
            return n;
        }
    }
    100
}

fn read(name: &str) -> std::io::Result<String> {
    if name == "-" {
        let mut s = String::new();
        std::io::stdin().read_to_string(&mut s)?;
        return Ok(s);
    }
    std::fs::read_to_string(name)
}

fn fail(msg: impl std::fmt::Display) -> ! {
    eprintln!("boxmaid: {msg}");
    exit(2)
}

fn main() {
    let mut f = parse_flags();
    if f.list {
        println!("{}", boxmaid::kinds().join("\n"));
        return;
    }
    let Some(theme) = Theme::by_name(&f.theme) else { fail(format!("unknown theme {:?}", f.theme)) };
    let tty = std::io::stdout().is_terminal();
    if f.width == 0 && tty {
        f.width = terminal_width();
    }
    let color = f.color == "always" || (f.color == "auto" && tty && std::env::var("NO_COLOR").unwrap_or_default().is_empty());
    let opt = Options { width: f.width, ascii: f.ascii };
    if f.inputs.is_empty() {
        f.inputs.push("-".into());
    }
    let mut status = 0;
    for input in &f.inputs {
        let src = read(input).unwrap_or_else(|e| fail(format!("{input}: {e}")));
        if f.live {
            println!("{}", boxmaid::live_url(&src));
        } else if f.md {
            print!("{}", boxmaid::replace_mermaid(&src, &opt));
        } else {
            match boxmaid::render(&src, &opt) {
                Ok(p) => {
                    if !p.title.is_empty() {
                        println!("{}\n", p.title);
                    }
                    if color {
                        println!("{}", p.ansi(&theme));
                    } else {
                        println!("{p}");
                    }
                }
                Err(e) => {
                    eprintln!("{input}: {e}");
                    status = 1;
                }
            }
        }
    }
    exit(status)
}
