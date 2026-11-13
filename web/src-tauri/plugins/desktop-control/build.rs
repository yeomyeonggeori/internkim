const COMMANDS: &[&str] = &[
    "state",
    "allow",
    "ask_once",
    "disallow",
    "run",
    "open_permission_guide",
];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).build();
}
