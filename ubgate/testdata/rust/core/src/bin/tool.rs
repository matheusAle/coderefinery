// src/bin/tool.rs: namespace `tool` (no crate prefix).
fn main() {
    let args: Vec<String> = std::env::args().collect();
    if args.len() > 1 {
        println!("{}", args[1]);
    }
}

fn helper(n: u32) -> u32 {
    (0..n).filter(|i| i % 2 == 0).sum::<u32>()
}
