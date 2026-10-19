const COMMANDS: &[&str] = &["install", "supply", "forget"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).build();
}
