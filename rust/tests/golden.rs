//! The shared corpus: ../testdata/cases/*.mmd, drawn in Unicode at width
//! 100 and in ASCII at width 80, must match ../testdata/golden exactly, the
//! same files the Go module checks, so the two implementations can't drift.
//! The Go tests write the golden files (go test -run Golden -update).

use std::fs;
use std::path::{Path, PathBuf};

use boxmaid::{render, Options};

fn testdata() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../testdata")
}

/// What a golden file holds for one rendering.
fn golden_text(src: &str, opt: &Options) -> String {
    match render(src, opt) {
        Ok(p) => format!("kind: {}\ntitle: {}\n\n{p}\n", p.kind, p.title),
        Err(e) => format!("error: {e}\n"),
    }
}

/// The first differing line, for a readable failure.
fn first_difference(got: &str, want: &str) -> String {
    let (g, w): (Vec<&str>, Vec<&str>) = (got.split('\n').collect(), want.split('\n').collect());
    for i in 0..g.len().max(w.len()) {
        let (a, b) = (g.get(i).copied().unwrap_or("<none>"), w.get(i).copied().unwrap_or("<none>"));
        if a != b {
            return format!("line {}:\n  got:  {a:?}\n  want: {b:?}", i + 1);
        }
    }
    String::new()
}

#[test]
fn golden() {
    let mut cases: Vec<PathBuf> = fs::read_dir(testdata().join("cases"))
        .expect("testdata/cases")
        .map(|e| e.unwrap().path())
        .filter(|p| p.extension().is_some_and(|e| e == "mmd"))
        .collect();
    cases.sort();
    assert!(!cases.is_empty(), "no cases");
    let mut failures = vec![];
    for c in &cases {
        let name = c.file_stem().unwrap().to_string_lossy();
        let src = fs::read_to_string(c).unwrap();
        for (suffix, opt) in [(".txt", Options { width: 100, ascii: false }), (".ascii.txt", Options { width: 80, ascii: true })] {
            let path = testdata().join("golden").join(format!("{name}{suffix}"));
            let Ok(want) = fs::read_to_string(&path) else {
                failures.push(format!("{}: no golden file (run go test -run Golden -update)", path.display()));
                continue;
            };
            let got = golden_text(&src, &opt);
            if got != want {
                failures.push(format!("{name}{suffix} differs: {}\n--- got\n{got}\n--- want\n{want}", first_difference(&got, &want)));
            }
        }
    }
    if !failures.is_empty() {
        panic!("{} of {} renderings differ:\n\n{}", failures.len(), cases.len() * 2, failures.join("\n\n"));
    }
}

#[test]
fn ascii_goldens_are_pure_ascii() {
    for e in fs::read_dir(testdata().join("golden")).unwrap() {
        let p = e.unwrap().path();
        if p.to_string_lossy().ends_with(".ascii.txt") {
            let s = fs::read_to_string(&p).unwrap();
            assert!(s.is_ascii(), "{}: non-ASCII text", p.display());
        }
    }
}
