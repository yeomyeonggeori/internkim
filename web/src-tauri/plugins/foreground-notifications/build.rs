const COMMANDS: &[&str] = &["hold_back"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).ios_path("ios").build();
}
