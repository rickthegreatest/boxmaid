//! The public API, mirroring the Go module's tests.

use boxmaid::{kind, kinds, live_url, render, replace_mermaid, supported, Error, Options, Role, Theme};

fn draw(src: &str, opt: Options) -> String {
    match render(src, &opt) {
        Ok(p) => p.to_string(),
        Err(e) => panic!("render: {e}\n{src}"),
    }
}

fn missing(out: &str, want: &[&str]) {
    for s in want {
        assert!(out.contains(s), "missing {s:?}:\n{out}");
    }
}

#[test]
fn subgraphs() {
    let out = draw("graph TD\n  X --> A1\n  subgraph one [First group]\n    A1 --> A2\n  end\n  A2 --> Y", Options::default());
    missing(&out, &["First group", "X", "A1", "A2", "Y", "▼"]);
}

/// The drawing never lets a line run through a node's text.
#[test]
fn edges_dont_cross_text() {
    let out = draw(
        "flowchart TD\n  A[Request arrives] --> B{Valid?}\n  B -- Yes --> C[Process]\n  B -- No --> D[Reject]\n  C --> E[(Store)]\n  D --> E\n  E --> A",
        Options::default(),
    );
    missing(&out, &["Request arrives", "Valid?", "Process", "Reject", "Store", "Yes", "No"]);
}

#[test]
fn fits_width() {
    let src = "flowchart LR\n  A[Collect all the signals] --> B[Resolve the recommendation] --> C[Persist the decision] --> D[Publish the outcome event] --> E[Notify the agent]";
    let wide = render(src, &Options::default()).unwrap();
    let narrow = render(src, &Options { width: 80, ascii: false }).unwrap();
    assert!(narrow.w <= 80 && narrow.w < wide.w, "width 80: got {} (unlimited {})", narrow.w, wide.w);
}

#[test]
fn linear_flowchart_stays_compact() {
    let p = render("flowchart LR\nA[Source]-->B[Layout]-->C[Text]", &Options::default()).unwrap();
    assert_eq!(p.h, 3, "{p}");
}

#[test]
fn sequence() {
    let out = draw(
        "sequenceDiagram\n  autonumber\n  actor U as Customer\n  participant A as API\n  U->>+A: POST /checkout\n  alt ok\n    A-->>U: 201\n  else failed\n    A--xU: 402\n  end\n  A->>A: retry\n  Note over U,A: idempotent\n  deactivate A",
        Options::default(),
    );
    missing(&out, &["Customer", "API", "① POST /checkout", "alt", "[ok]", "else", "② 201", "×", "retry", "idempotent", "►", "◄"]);
}

#[test]
fn state_c4_and_pie() {
    let st = draw("stateDiagram-v2\n  [*] --> Idle\n  Idle --> Busy : job\n  state Busy {\n    [*] --> Working\n  }\n  Busy --> [*]", Options::default());
    missing(&st, &["●", "◉", "Idle", "job", "Busy", "Working"]);
    let c4 = draw(
        "C4Context\n  Person(u, \"User\", \"A customer\")\n  System_Boundary(b, \"Shop\") {\n    System(s, \"Shop API\", \"Takes orders\")\n  }\n  Rel(u, s, \"Uses\", \"HTTPS\")",
        Options::default(),
    );
    missing(&c4, &["User", "[Person]", "Shop API", "Takes orders", "Uses", "[HTTPS]", "Shop"]);
    let pie = draw("pie title Pets\n  \"Dogs\" : 3\n  \"Cats\" : 1", Options::default());
    missing(&pie, &["Dogs", "75.0%"]);
}

#[test]
fn ascii() {
    let out = draw("flowchart TD\n  A --> B{x?}\n  B -.->|no| C((c))", Options { width: 0, ascii: true });
    assert!(out.is_ascii(), "non-ASCII in:\n{out}");
}

#[test]
fn unsupported_and_broken() {
    assert_eq!(render("zenuml\n  title Order\n  Alice->Bob: hi", &Options::default()), Err(Error::Unsupported("zenuml".into())));
    assert_eq!(render("", &Options::default()), Err(Error::Empty));
    // Nothing malformed may panic.
    for src in [
        "flowchart TD\n  A -->",
        "flowchart\n  A[unclosed --> B",
        "graph LR\n  subgraph\n  end\n  end",
        "sequenceDiagram\n  end\n  else",
        "stateDiagram\n  }\n  [*] --> [*]",
        "C4Context\n  Rel(a, b)",
        "flowchart TD\n  A --> A\n  A --> A",
        "pie",
        "flowchart TD\n  subgraph s\n    a --> b\n  end\n  b --> s\n  s --> a",
        "requirementDiagram\n  requirement r {\n  risk: \"\"\n  }",
        "architecture-beta\n  group g in g\n  service s in g",
    ] {
        let _ = render(src, &Options { width: 40, ascii: false });
    }
}

#[test]
fn front_matter_and_directives() {
    let p =
        render("---\ntitle: My flow\nconfig:\n  theme: dark\n---\n%%{init: {'theme':'dark'}}%%\nflowchart TD\n  %% a comment\n  A --> B", &Options::default())
            .unwrap();
    assert_eq!(p.title, "My flow");
    assert!(p.to_string().contains('A'));
}

#[test]
fn kinds_and_support() {
    assert!(kinds().contains(&"flowchart") && kinds().contains(&"sequenceDiagram"));
    assert_eq!(kind("%% hi\ngraph LR\n A-->B"), "graph");
    assert!(supported("erDiagram\n  A ||--o{ B : has"));
    assert!(!supported("zenuml"));
}

#[test]
fn live_url_carries_the_source() {
    let u = live_url("flowchart TD\n  A --> B");
    let enc = u.strip_prefix("https://mermaid.live/view#pako:").expect("prefix");
    assert!(enc.bytes().all(|b| b.is_ascii_alphanumeric() || b == b'-' || b == b'_'));
}

#[test]
fn markdown_blocks() {
    let md = "# Doc\n\n```mermaid\nflowchart LR\n  A --> B\n```\n\n```mermaid\nzenuml\n```\n";
    let out = replace_mermaid(md, &Options::default());
    assert!(out.contains("```text\n"), "{out}");
    assert!(out.contains("```mermaid\nzenuml\n```"), "unsupported blocks stay: {out}");
}

#[test]
fn markdown_fence_boundaries() {
    for md in [
        "````markdown\n```mermaid\nflowchart LR\nA-->B\n```\n````\n",
        "```mermaid-example\nflowchart LR\nA-->B\n```\n",
        "```mermaid\nflowchart LR\nA-->B\n~~~\n",
        "````mermaid\nflowchart LR\nA-->B\n```\n",
        "```mermaid\nflowchart LR\nA-->B\n",
        "```mermaid\nzenuml\n```\n",
    ] {
        assert_eq!(replace_mermaid(md, &Options::default()), md);
    }
    let md = "Intro\r\n\r\n  ~~~mermaid\r\n  flowchart LR\r\n  A-->B\r\n  ~~~~\r\nAfter\r\n";
    let got = replace_mermaid(md, &Options::default());
    assert!(got.contains("  ~~~text\r\n"), "{got:?}");
    assert!(got.ends_with("  ~~~~\r\nAfter\r\n"), "{got:?}");
    assert!(!got.replace("\r\n", "").contains('\n'), "{got:?}");
}

#[test]
fn pie_labels_and_large_values() {
    let name = "界".repeat(20);
    let got = draw(&format!("pie\n\"{name}\": 1"), Options::default());
    assert!(got.contains(&("界".repeat(14) + "…")), "{got}");
    assert!(!got.contains('\u{fffd}'), "{got}");
    let large = "9".repeat(308);
    let got = draw(&format!("pie\n\"A\": {large}\n\"B\": {large}"), Options::default());
    assert_eq!(got.matches("50.0%").count(), 2, "{got}");
}

#[test]
fn ansi_colours() {
    let p = render("flowchart LR\n  A --> B", &Options::default()).unwrap();
    let out = p.ansi(&Theme::default());
    assert!(out.contains("\x1b[34m"), "node borders are blue: {out:?}");
    assert_eq!(p.ansi(&Theme::by_name("mono").unwrap()).lines().count(), p.h);
    assert!(Theme::default().styles.contains_key(&Role::Warn));
    assert!(Theme::names().iter().all(|n| Theme::by_name(n).is_some()));
    let mut cell = boxmaid::Cell::default();
    cell.role = Role::Node;
    cell.color = "#123456".into();
    cell.bold = true;
    assert_eq!(Theme::nord().cell_style(&cell).color, "#123456");
    assert_eq!(Theme::default().cell_style(&cell).color, "4");
    assert!(Theme::default().cell_style(&cell).bold);
}
