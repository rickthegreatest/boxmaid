//! Dates for gantt charts, as milliseconds since 1970 in UTC, with the
//! calendar done by hand (days from civil and back, after Howard Hinnant)
//! so Go and Rust compute exactly the same dates.

pub(crate) const MS_SECOND: i64 = 1000;
pub(crate) const MS_MINUTE: i64 = 60 * MS_SECOND;
pub(crate) const MS_HOUR: i64 = 60 * MS_MINUTE;
pub(crate) const MS_DAY: i64 = 24 * MS_HOUR;

fn floor_div(a: i64, b: i64) -> i64 {
    let q = a / b;
    if a % b != 0 && ((a < 0) != (b < 0)) {
        q - 1
    } else {
        q
    }
}

/// The day number of a date (month and day may overflow; they roll over
/// like a calendar would).
pub(crate) fn days_from_civil(mut y: i64, mut m: i64, d: i64) -> i64 {
    // Fold the month into 1..12 first.
    y += floor_div(m - 1, 12);
    m = (m - 1) % 12 + 1;
    if m <= 0 {
        m += 12;
    }
    if m <= 2 {
        y -= 1;
    }
    let era = floor_div(y, 400);
    let yoe = y - era * 400;
    let mp = (m + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

pub(crate) fn civil_from_days(mut z: i64) -> (i64, i64, i64) {
    z += 719468;
    let era = floor_div(z, 146097);
    let doe = z - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let mut y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let mut m = mp + 3;
    if m > 12 {
        m -= 12;
    }
    if m <= 2 {
        y += 1;
    }
    (y, m, d)
}

/// 0 for Sunday … 6 for Saturday.
pub(crate) fn weekday(ms: i64) -> i64 {
    ((floor_div(ms, MS_DAY) % 7) + 7 + 4) % 7 // 1970-01-01 was a Thursday
}

pub(crate) fn date_ms(y: i64, mo: i64, d: i64, h: i64, mi: i64, s: i64, milli: i64) -> i64 {
    days_from_civil(y, mo, d) * MS_DAY + h * MS_HOUR + mi * MS_MINUTE + s * MS_SECOND + milli
}

/// Moves a date by whole months, rolling an overflowing day over (Jan 31 +
/// 1 month is Mar 2 or 3).
pub(crate) fn add_months(ms: i64, months: i64) -> i64 {
    let days = floor_div(ms, MS_DAY);
    let rest = ms - days * MS_DAY;
    let (y, m, d) = civil_from_days(days);
    days_from_civil(y, m + months, d) * MS_DAY + rest
}

/// Reads s in a Moment.js-style format ("YYYY-MM-DD", "DD/MM/YYYY HH:mm",
/// "X" for unix seconds, "x" for milliseconds).
pub(crate) fn parse_moment_date(s: &str, format: &str) -> Option<i64> {
    if format == "X" || format == "x" {
        let n: i64 = s.parse().ok()?;
        return Some(if format == "X" { n * MS_SECOND } else { n });
    }
    let (sb, fb) = (s.as_bytes(), format.as_bytes());
    let (mut y, mut mo, mut d) = (1970i64, 1i64, 1i64);
    let (mut h, mut mi, mut sec, mut milli) = (0i64, 0i64, 0i64, 0i64);
    let mut i = 0usize;
    let num = |i: &mut usize, max_len: usize| -> Option<i64> {
        let mut j = *i;
        while j < sb.len() && j - *i < max_len && sb[j].is_ascii_digit() {
            j += 1;
        }
        if j == *i {
            return None;
        }
        let n = s[*i..j].parse().unwrap_or(0);
        *i = j;
        Some(n)
    };
    let mut f = 0usize;
    while f < fb.len() {
        let tok = ["YYYY", "SSS", "YY", "MM", "DD", "Do", "HH", "mm", "ss", "M", "D", "H"]
            .into_iter()
            .find(|t| fb.len() - f >= t.len() && &fb[f..f + t.len()] == t.as_bytes());
        let Some(tok) = tok else {
            if i >= sb.len() || sb[i] != fb[f] {
                return None;
            }
            i += 1;
            f += 1;
            continue;
        };
        f += tok.len();
        match tok {
            "YYYY" => y = num(&mut i, 4)?,
            "YY" => y = 2000 + num(&mut i, 2)?,
            "MM" | "M" => mo = num(&mut i, 2)?,
            "DD" | "D" | "Do" => {
                d = num(&mut i, 2)?;
                if tok == "Do" && i + 2 <= sb.len() {
                    i += 2; // "st", "nd", "rd", "th"
                }
            }
            "HH" | "H" => h = num(&mut i, 2)?,
            "mm" => mi = num(&mut i, 2)?,
            "ss" => sec = num(&mut i, 2)?,
            _ => milli = num(&mut i, 3)?,
        }
    }
    if i != sb.len() || !(1..=12).contains(&mo) || !(1..=31).contains(&d) {
        return None;
    }
    Some(date_ms(y, mo, d, h, mi, sec, milli))
}

const MONTH_NAMES: [&str; 12] = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

fn pad2(n: i64) -> String {
    if n < 10 {
        format!("0{n}")
    } else {
        n.to_string()
    }
}

/// An axis label: "Jan 02", "15:04" or "Jan 2006".
pub(crate) fn format_axis_date(ms: i64, style: &str) -> String {
    let days = floor_div(ms, MS_DAY);
    let (y, m, d) = civil_from_days(days);
    let rest = ms - days * MS_DAY;
    match style {
        "time" => format!("{}:{}", pad2(rest / MS_HOUR), pad2(rest % MS_HOUR / MS_MINUTE)),
        "month" => format!("{} {y}", MONTH_NAMES[(m - 1) as usize]),
        _ => format!("{} {}", MONTH_NAMES[(m - 1) as usize], pad2(d)),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dates() {
        for (y, m, d) in [(1970, 1, 1), (2000, 2, 29), (2024, 12, 31), (1969, 7, 20), (2100, 3, 1)] {
            assert_eq!(civil_from_days(days_from_civil(y, m, d)), (y, m, d));
        }
        assert_eq!(weekday(date_ms(2024, 1, 6, 0, 0, 0, 0)), 6);
        assert_eq!(weekday(date_ms(1970, 1, 1, 0, 0, 0, 0)), 4);
        assert_eq!(add_months(date_ms(2024, 1, 31, 0, 0, 0, 0), 1), date_ms(2024, 3, 2, 0, 0, 0, 0));
        assert_eq!(parse_moment_date("17/10/2026 14:05", "DD/MM/YYYY HH:mm"), Some(date_ms(2026, 10, 17, 14, 5, 0, 0)));
        assert_eq!(parse_moment_date("2026-13-01", "YYYY-MM-DD"), None);
        assert_eq!(format_axis_date(date_ms(2026, 10, 7, 9, 3, 0, 0), "day"), "Oct 07");
        assert_eq!(format_axis_date(date_ms(2026, 10, 7, 9, 3, 0, 0), "time"), "09:03");
        assert_eq!(floor_div(-1, 12), -1);
        assert_eq!(floor_div(-12, 12), -1);
        assert_eq!(floor_div(7, 2), 3);
    }
}
