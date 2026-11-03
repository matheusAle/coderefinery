// app-cli has no package name: the crate is the directory, app_cli.
mod commands {
    pub fn run(cmd: &str) -> i32 {
        match cmd {
            "help" => 0,
            "version" if cfg!(feature = "full") => 1,
            _ => 2,
        }
    }
}

fn main() {
    std::process::exit(commands::run("help"));
}
