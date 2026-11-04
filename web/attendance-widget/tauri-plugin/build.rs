const COMMANDS: &[&str] = &[
    "install",
    "supply",
    "forget",
    "background_refresh_state",
    "open_settings",
];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).ios_path("ios").build();
}
