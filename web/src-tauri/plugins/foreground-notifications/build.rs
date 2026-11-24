const COMMANDS: &[&str] = &["hold_back", "clear_delivered"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).ios_path("ios").build();
}
