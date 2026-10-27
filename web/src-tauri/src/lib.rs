#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let builder = tauri::Builder::default().plugin(tauri_plugin_os::init());
    #[cfg(mobile)]
    let builder = builder
        .plugin(tauri_plugin_attendance_widget::init())
        .plugin(tauri_plugin_page_theme::init())
        .plugin(tauri_plugin_keyboard::init())
        .plugin(tauri_plugin_haptics::init());
    #[cfg(any(target_os = "android", target_os = "ios", target_os = "macos"))]
    let builder = builder.plugin(tauri_plugin_notifications::init());
    #[cfg(mobile)]
    let builder = builder
        .plugin(tauri_plugin_foreground_notifications::init())
        .plugin(tauri_plugin_silent_push::init())
        .plugin(tauri_plugin_attendance_activity::init())
        .plugin(tauri_plugin_back_gesture::init());
    builder
        .run(tauri::generate_context!())
        .expect("the internkim app could not start");
}
