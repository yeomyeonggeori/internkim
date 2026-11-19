const COMMANDS: &[&str] = &["save", "save_download"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).ios_path("ios").build();
}
