const COMMANDS: &[&str] = &["save"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).build();
}
