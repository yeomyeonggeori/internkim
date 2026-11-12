const COMMANDS: &[&str] = &["state", "allow", "disallow", "run", "open_permission_guide"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).build();
}
