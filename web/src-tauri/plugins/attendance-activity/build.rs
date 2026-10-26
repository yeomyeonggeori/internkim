const COMMANDS: &[&str] = &[
    "register_listener",
    "remove_listener",
    "watch_tokens",
    "held_tokens",
    "lock_screen_state",
    "close_cards",
];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).ios_path("ios").build();
}
