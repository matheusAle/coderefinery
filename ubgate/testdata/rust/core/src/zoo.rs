/* Lexer and grammar zoo.
   /* nested block comment */ still a comment
*/
#![allow(unused)]

use std::borrow::Cow;
use std::{self, collections::BTreeMap as Map, io::*};

/// Lifetimes vs char literals.
pub fn chars<'a, 'b: 'a>(s: &'a str, t: &'b str) -> (char, char, char, &'a str) {
    let quote = '\'';
    let brace = '{';
    let emoji = '\u{1F600}';
    let byte = b'x';
    let _ = (byte, t);
    if s.starts_with('a') && quote != brace {
        return (quote, brace, emoji, s);
    }
    ('a', 'b', 'c', s)
}

pub fn strings() -> Vec<&'static str> {
    let raw = r"C:\path\no\escape";
    let hashed = r#"a "quoted" word"#;
    let more = r##"has "# inside"##;
    let bytes = b"bytes\n\x7f";
    let raw_bytes = br#"raw bytes"#;
    let escaped = "tab\tquote\"backslash\\unicode\u{e9}";
    let continued = "line one \
                     line two";
    let _ = (bytes, raw_bytes);
    vec![raw, hashed, more, escaped, continued]
}

pub fn numbers(t: (u8, (u16, u32))) -> f64 {
    let range = 1..2;
    let inclusive = 0..=9;
    let tuple_index = t.1.0;
    let suffixed = 1u8 as u32 + 0x_FF + 0o17 + 0b1010_1010 + 1_000;
    let floats = 1e3 + 2.5e-3 + 3.0f64 + 4. + 1_000.000_1;
    let _ = (range, inclusive);
    floats + suffixed as f64 + tuple_index as f64 + 1.max(2) as f64
}

pub fn closures(xs: &[i32]) -> impl Fn(i32) -> bool + '_ {
    let add = |a: i32, b: i32| -> i32 { a + b };
    let pick = |x| if x > 0 { 1 } else { 2 };
    let empty = || 0;
    let moved = move |v: &i32| *v > add(1, 2) && *v < pick(3);
    let nested = |f: &dyn Fn(i32) -> i32| f(empty());
    let _ = nested(&|x| x * 2);
    move |x| xs.iter().any(|y| *y == x || moved(y))
}

pub async fn asyncs(input: Option<u32>) -> Option<u32> {
    let fut = async { input? + 1 };
    let fut2 = async move { 2 };
    let value = fut.await? + fut2.await;
    Some(value)
}

pub fn generics<T, U>(items: Vec<T>) -> Map<String, Vec<U>>
where
    T: Into<U> + Clone,
    U: Default + std::fmt::Debug,
{
    let parsed = "42".parse::<i64>().unwrap_or_default();
    let collected: Vec<U> = items.into_iter().map(Into::into).collect::<Vec<_>>();
    let mut map = Map::<String, Vec<U>>::new();
    if parsed > 0 {
        map.insert(String::from("k"), collected);
    }
    map
}

pub fn patterns(v: &[Option<(i32, char)>]) -> i32 {
    let mut score = 0;
    for item in v {
        score += match item {
            Some((n @ 1..=9, 'a'..='z')) => *n,
            Some((-1, _)) | None => -1,
            Some((n, c)) if *c == 'x' && *n > 100 => 2,
            Some((ref n, _)) => *n % 7,
        };
    }
    let [first, .., last] = [1, 2, 3, 4];
    let (a, mut b) = (first, last);
    b += a;
    score + b
}

#[derive(Default)]
pub struct Config<'a> {
    pub name: Cow<'a, str>,
    pub(crate) retries: u8,
    flags: [bool; 4],
}

impl<'a> Config<'a> {
    pub const LIMIT: u8 = 3;

    pub fn builder() -> Self {
        Self { retries: Self::LIMIT, ..Default::default() }
    }

    pub fn retry<F, E>(&self, mut op: F) -> Result<u8, E>
    where
        F: FnMut(u8) -> Result<u8, E>,
    {
        let mut attempt = 0;
        loop {
            match op(attempt) {
                Ok(v) => break Ok(v),
                Err(e) if attempt >= self.retries => return Err(e),
                Err(_) => attempt += 1,
            }
        }
    }

    pub unsafe fn raw_flags(&self) -> *const bool {
        self.flags.as_ptr()
    }
}

pub fn labeled(grid: &[Vec<u8>]) -> Option<(usize, usize)> {
    'outer: for (i, row) in grid.iter().enumerate() {
        for (j, &cell) in row.iter().enumerate() {
            if cell == 0 {
                continue 'outer;
            }
            if cell == 9 {
                return Some((i, j));
            }
        }
    }
    let found = 'search: {
        if grid.is_empty() {
            break 'search None;
        }
        Some((0, 0))
    };
    found
}

pub fn statements(mut n: u32) -> u32 {
    let Some(start) = n.checked_sub(1) else {
        return 0;
    };
    while n > start / 2 {
        n -= 1;
    }
    unsafe {
        n = n.wrapping_add(1);
    }
    if n % 2 == 0 {} else if n % 3 == 0 {} else {}
    let t = if n > 10 { n } else { start }.max(1);
    let q = match n { 0 => 1, x => x }.pow(2);
    n + t + q
}

extern "C" {
    fn abs(input: i32) -> i32;
}

pub static GREETING: &str = "hi";
static mut COUNTER: u32 = 0;
type Pair<T> = (T, T);

pub union Bits {
    int: u32,
    float: f32,
}

pub trait Visitor {
    type Output;
    const DEPTH: usize = 1;
    fn visit(&mut self, n: u32) -> Self::Output;
    fn finish(self) -> Self::Output
    where
        Self: Sized;
}
