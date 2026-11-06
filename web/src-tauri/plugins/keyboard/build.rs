const COMMANDS: &[&str] = &["register_listener", "remove_listener", "set_accessory_bar"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS).ios_path("ios").build();
}
