use tauri::{
    plugin::{Builder, TauriPlugin},
    Runtime,
};

#[cfg(target_os = "macos")]
mod macos;

#[cfg(target_os = "ios")]
tauri::ios_plugin_binding!(init_plugin_foreground_notifications);

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    let builder = Builder::new("foreground-notifications");
    #[cfg(target_os = "macos")]
    let builder = builder.invoke_handler(tauri::generate_handler![macos::hold_back]);
    builder
        .setup(|_app, _api| {
            #[cfg(target_os = "ios")]
            _api.register_ios_plugin(init_plugin_foreground_notifications)?;
            #[cfg(target_os = "macos")]
            macos::present_pushes_while_the_app_is_open(_app);
            Ok(())
        })
        .build()
}
