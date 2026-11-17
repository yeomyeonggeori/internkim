const COMMANDS: &[&str] = &["present"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).ios_path("ios").build();
}
