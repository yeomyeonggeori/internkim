#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let builder = tauri::Builder::default().plugin(tauri_plugin_os::init());
    #[cfg(mobile)]
    let builder = builder.plugin(tauri_plugin_attendance_widget::init());
    builder
        .run(tauri::generate_context!())
        .expect("the internkim app could not start");
}
