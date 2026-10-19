use tauri::{
    plugin::{Builder, TauriPlugin},
    Runtime,
};

#[cfg(target_os = "android")]
const ANDROID_PACKAGE: &str = "app.intern.kim.attendance";
#[cfg(target_os = "android")]
const ANDROID_CLASS: &str = "AttendanceWidgetPlugin";

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("attendance-widget")
        .setup(|_app, _api| {
            #[cfg(target_os = "android")]
            _api.register_android_plugin(ANDROID_PACKAGE, ANDROID_CLASS)?;
            Ok(())
        })
        .build()
}
