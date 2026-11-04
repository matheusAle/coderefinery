pub mod game;

pub fn choose(x: i32) -> i32 {
    if x > 0 && x < 10 {
        return 1;
    }
    match x {
        1 => 1,
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

mod tests {
    fn hidden() {
        if true {}
    }
}
