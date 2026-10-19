use tauri::{
    plugin::{Builder, TauriPlugin},
    Runtime,
};

#[cfg(target_os = "android")]
const ANDROID_PACKAGE: &str = "app.intern.kim.attendance";
#[cfg(target_os = "android")]
const ANDROID_CLASS: &str = "AttendanceWidgetPlugin";

#[cfg(target_os = "ios")]
tauri::ios_plugin_binding!(init_plugin_attendance_widget);

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("attendance-widget")
        .setup(|_app, _api| {
            #[cfg(target_os = "android")]
            _api.register_android_plugin(ANDROID_PACKAGE, ANDROID_CLASS)?;
            #[cfg(target_os = "ios")]
            _api.register_ios_plugin(init_plugin_attendance_widget)?;
            Ok(())
        })
        .build()
}
