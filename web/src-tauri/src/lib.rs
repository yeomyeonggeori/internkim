#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let builder = tauri::Builder::default().plugin(tauri_plugin_os::init());
    #[cfg(mobile)]
    let builder = builder
        .plugin(tauri_plugin_attendance_widget::init())
        .plugin(tauri_plugin_page_theme::init())
        .plugin(tauri_plugin_keyboard::init())
        .plugin(tauri_plugin_haptics::init());
    #[cfg(target_os = "android")]
    let builder = builder.plugin(tauri_plugin_notifications::init());
    builder
        .run(tauri::generate_context!())
        .expect("the internkim app could not start");
}
