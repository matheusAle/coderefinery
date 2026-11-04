//! Crate root: the upstream test SOURCE plus items at the crate level.
#![allow(dead_code)]

pub mod game;
mod macros;
mod zoo;

pub fn choose(x: i32) -> i32 {
    if x > 0 && x < 10 {
        return 1;
    }
    match x {
        1 => 1,
        2 => 2,
        _ => 0,
    }
}

pub struct Sample {
    ok: bool,
}

impl Sample {
    pub fn run(&self) -> i32 {
        if self.ok { 1 } else { 0 }
    }
}

pub trait Trait {
    fn draw(&mut self);

    /// Default methods have no impl type.
    fn describe(&self) -> &'static str {
        if cfg!(debug_assertions) { "debug" } else { "release" }
    }
}

impl Trait for Sample {
    fn draw(&mut self) {}
}

mod tests {
    fn hidden() {
        if true {}
    }

    mod deeper {
        fn also_hidden(a: bool) -> bool { a || !a }
    }
}

#[cfg(test)]
mod test {
    fn counted(v: Option<u8>) -> Option<u8> {
        let x = v?;
        Some(x)
    }
}

fn outer() {
    fn inner() {
        if true {}
    }
    struct Local;
    impl Local {
        fn method(&self) -> u8 {
            if inner_ready() { 1 } else { 2 }
        }
    }
}

fn inner_ready() -> bool {
    true
}

pub const unsafe extern "C" fn qualified(n: u32) -> u32 {
    n
}

pub(crate) async fn fetch(urls: &[&str]) -> Result<usize, String> {
    let mut total = 0;
    for url in urls {
        if url.is_empty() {
            continue;
        }
        total += url.len();
    }
    Ok(total)
}
