#[cfg(target_os = "macos")]
mod macos_window_lifecycle;
#[cfg(desktop)]
mod window_background;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let builder = tauri::Builder::default()
        .plugin(tauri_plugin_os::init())
        .plugin(tauri_plugin_opener::init());
    #[cfg(mobile)]
    let builder = builder
        .plugin(tauri_plugin_attendance_widget::init())
        .plugin(tauri_plugin_page_theme::init())
        .plugin(tauri_plugin_keyboard::init())
        .plugin(tauri_plugin_haptics::init());
    #[cfg(desktop)]
    let builder = builder
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_desktop_control::init())
        .plugin(tauri_plugin_file_save::init())
        .setup(window_background::paint_every_window)
        .on_window_event(window_background::repaint_on_appearance_change);
    #[cfg(target_os = "macos")]
    let builder =
        builder.on_window_event(macos_window_lifecycle::hide_main_window_instead_of_closing);
    #[cfg(any(target_os = "android", target_os = "ios", target_os = "macos"))]
    let builder = builder
        .plugin(tauri_plugin_notifications::init())
        .plugin(tauri_plugin_foreground_notifications::init());
    #[cfg(mobile)]
    let builder = builder
        .plugin(tauri_plugin_silent_push::init())
        .plugin(tauri_plugin_attendance_activity::init())
        .plugin(tauri_plugin_back_gesture::init())
        .plugin(tauri_plugin_print_sheet::init());
    let app = builder
        .build(tauri::generate_context!())
        .expect("the internkim app could not start");
    app.run(|app_handle, event| {
        #[cfg(target_os = "macos")]
        macos_window_lifecycle::show_main_window_on_reopen(app_handle, event);
        #[cfg(not(target_os = "macos"))]
        let _ = (app_handle, event);
    });
}
