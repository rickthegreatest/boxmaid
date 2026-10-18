//! Command-line adapter for the boxmaid rendering library.

use std::io::{self, IsTerminal, Read, Write};
use std::process::{Command, Stdio};

use boxmaid::{Options, Theme};

#[derive(Default)]
struct Flags {
    width: usize,
    ascii: bool,
    color: String,
    theme: String,
    md: bool,
    live: bool,
    list: bool,
    version: bool,
    help: bool,
    require_fit: bool,
    inputs: Vec<String>,
}

fn usage() -> String {
    format!(
        "Usage: boxmaid [flags] [file …]

Read stdin when no files are given; use - for stdin or -- before filenames starting with -.
Place flags before filenames. Width fitting is best-effort unless -require-fit is set.

  -w int           column budget; 0 uses terminal width, or unlimited when piped
  -ascii           draw with plain ASCII characters
  -color string    auto, always or never (default auto)
  -theme string    {} (default default)
  -md              replace fenced Mermaid blocks in Markdown with plain text
  -live            print a mermaid.live URL; does not open a browser
  -list            list supported diagram headers
  -version         print the version
  -require-fit     exit 3 without drawing if the diagram exceeds the column budget
  -help            show this help

Examples:
  boxmaid diagram.mmd
  boxmaid -w 80 -color never diagram.mmd
  boxmaid -md README.md

Exit codes: 0 success, 1 render/output error, 2 usage/input error, 3 cannot fit.",
        Theme::names().join(", ")
    )
}

fn boolean(value: Option<&str>, name: &str) -> Result<bool, String> {
    match value {
        None | Some("1" | "t" | "T" | "true" | "TRUE" | "True") => Ok(true),
        Some("0" | "f" | "F" | "false" | "FALSE" | "False") => Ok(false),
        Some(v) => Err(format!("invalid boolean value {v:?} for -{name}")),
    }
}

// Match Go's flag convention: stop at the first filename; support --name and -name=value.
fn parse_flags(mut args: impl Iterator<Item = String>) -> Result<Flags, String> {
    let mut f = Flags { color: "auto".into(), theme: "default".into(), ..Default::default() };
    while let Some(a) = args.next() {
        if a == "--" {
            break;
        }
        if a == "-" || !a.starts_with('-') {
            f.inputs.push(a);
            break;
        }
        let name = a.strip_prefix("--").or_else(|| a.strip_prefix('-')).unwrap_or(&a);
        let (name, value) = name.split_once('=').map_or((name, None), |(n, v)| (n, Some(v)));
        match name {
            "ascii" => f.ascii = boolean(value, name)?,
            "md" => f.md = boolean(value, name)?,
            "live" => f.live = boolean(value, name)?,
            "list" => f.list = boolean(value, name)?,
            "version" => f.version = boolean(value, name)?,
            "require-fit" => f.require_fit = boolean(value, name)?,
            "h" | "help" => {
                f.help = true;
                return Ok(f);
            }
            "w" | "color" | "theme" => {
                let value = value.map(str::to_string).or_else(|| args.next()).ok_or_else(|| format!("flag needs an argument: -{name}"))?;
                match name {
                    "w" => f.width = value.parse().map_err(|_| "-w must be a non-negative integer".to_string())?,
                    "color" => f.color = value,
                    "theme" => f.theme = value,
                    _ => unreachable!(),
                }
            }
            _ => return Err(format!("unknown flag -{name}; use -help for examples")),
        }
    }
    f.inputs.extend(args);
    Ok(f)
}

fn validate(f: &Flags) -> Result<Theme, String> {
    if !matches!(f.color.as_str(), "auto" | "always" | "never") {
        return Err(format!("unknown colour mode {:?}; use auto, always or never", f.color));
    }
    if f.md && f.live || f.require_fit && (f.md || f.live) {
        return Err("-md, -live and -require-fit cannot be combined".into());
    }
    Theme::by_name(&f.theme).ok_or_else(|| format!("unknown theme {:?}; use {}", f.theme, Theme::names().join(", ")))
}

/// Prefer COLUMNS; stty is optional and a missing utility falls back to 100.
fn terminal_width() -> usize {
    if let Some(n) = std::env::var("COLUMNS").ok().and_then(|c| c.trim().parse().ok()).filter(|&n| n > 0) {
        return n;
    }
    let tty = std::fs::File::open("/dev/tty").map(Stdio::from).unwrap_or_else(|_| Stdio::inherit());
    if let Ok(out) = Command::new("stty").arg("size").stdin(tty).stderr(Stdio::null()).output() {
        if let Some(n) = String::from_utf8_lossy(&out.stdout).split_whitespace().nth(1).and_then(|c| c.parse().ok()).filter(|&n: &usize| n > 0) {
            return n;
        }
    }
    100
}

fn read(name: &str) -> io::Result<String> {
    if name == "-" {
        let mut s = String::new();
        io::stdin().read_to_string(&mut s)?;
        return Ok(s);
    }
    std::fs::read_to_string(name)
}

fn run(mut f: Flags, theme: Theme, out: &mut impl Write) -> io::Result<i32> {
    if f.list {
        writeln!(out, "{}", boxmaid::kinds().join("\n"))?;
        return Ok(0);
    }
    if f.version {
        writeln!(out, "boxmaid {}", env!("CARGO_PKG_VERSION"))?;
        return Ok(0);
    }
    let tty = io::stdout().is_terminal();
    if f.width == 0 && tty {
        f.width = terminal_width();
    }
    if f.require_fit && f.width == 0 {
        eprintln!("boxmaid: -require-fit needs -w or a terminal with a known width");
        return Ok(2);
    }
    if f.inputs.is_empty() {
        if io::stdin().is_terminal() {
            eprintln!("boxmaid: provide a file or pipe Mermaid source; use -help for examples");
            return Ok(2);
        }
        f.inputs.push("-".into());
    }
    let color = f.color == "always" || (f.color == "auto" && tty && std::env::var("NO_COLOR").unwrap_or_default().is_empty());
    let opt = Options { width: f.width, ascii: f.ascii };
    let mut status = 0;
    for input in &f.inputs {
        let src = match read(input) {
            Ok(src) => src,
            Err(e) => {
                eprintln!("boxmaid: {input}: {e}");
                status = status.max(2);
                continue;
            }
        };
        let output = if f.live {
            format!("{}\n", boxmaid::live_url(&src))
        } else if f.md {
            boxmaid::replace_mermaid(&src, &opt)
        } else {
            match boxmaid::render(&src, &opt) {
                Ok(p) => {
                    if opt.width > 0 && p.w > opt.width {
                        eprintln!("boxmaid: {input}: diagram is {} columns; budget is {}", p.w, opt.width);
                        if f.require_fit {
                            status = 3;
                            continue;
                        }
                    }
                    let title = if p.title.is_empty() { String::new() } else { format!("{}\n\n", p.title) };
                    format!("{title}{}\n", if color { p.ansi(&theme) } else { p.to_string() })
                }
                Err(e) => {
                    eprintln!("boxmaid: {input}: {e}");
                    status = status.max(1);
                    continue;
                }
            }
        };
        out.write_all(output.as_bytes())?;
    }
    Ok(status)
}

fn main() {
    let f = match parse_flags(std::env::args().skip(1)) {
        Ok(f) => f,
        Err(e) => {
            eprintln!("boxmaid: {e}");
            std::process::exit(2);
        }
    };
    if f.help {
        eprintln!("{}", usage());
        return;
    }
    let theme = match validate(&f) {
        Ok(t) => t,
        Err(e) => {
            eprintln!("boxmaid: {e}");
            std::process::exit(2);
        }
    };
    let status = match run(f, theme, &mut io::stdout().lock()) {
        Ok(status) => status,
        Err(e) if e.kind() == io::ErrorKind::BrokenPipe => 0,
        Err(e) => {
            eprintln!("boxmaid: {e}");
            1
        }
    };
    std::process::exit(status);
}
