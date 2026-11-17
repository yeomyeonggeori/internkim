use tauri::{
    plugin::{Builder, TauriPlugin},
    Runtime,
};

#[cfg(target_os = "ios")]
tauri::ios_plugin_binding!(init_plugin_print_sheet);

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("print-sheet")
        .setup(|_app, _api| {
            #[cfg(target_os = "ios")]
            _api.register_ios_plugin(init_plugin_print_sheet)?;
            Ok(())
        })
        .build()
}
