// src/bin/admin/main.rs: namespace `admin::main`.
fn main() {
    let level = std::env::var("LEVEL").unwrap_or_default();
    match level.as_str() {
        "debug" | "trace" => println!("verbose"),
        "" => {}
        other => eprintln!("unknown level {other}"),
    }
}
