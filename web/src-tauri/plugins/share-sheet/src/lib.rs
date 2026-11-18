use serde::Serialize;
use tauri::{
    ipc::Request,
    plugin::{Builder, PluginHandle, TauriPlugin},
    AppHandle, Manager, Runtime, State,
};

#[cfg(target_os = "android")]
const ANDROID_PACKAGE: &str = "app.intern.kim";
#[cfg(target_os = "android")]
const ANDROID_CLASS: &str = "ShareSheetPlugin";

#[cfg(target_os = "ios")]
tauri::ios_plugin_binding!(init_plugin_share_sheet);

const RECEIVING_FOLDER: &str = "share-sheet";

struct ShareSheet<R: Runtime>(PluginHandle<R>);

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SharedFile {
    path: String,
    media_type: String,
}

#[tauri::command]
async fn present<R: Runtime>(app: AppHandle<R>, sheet: State<'_, ShareSheet<R>>, request: Request<'_>) -> Result<(), String> {
    let folder = app
        .path()
        .app_cache_dir()
        .map_err(|error| format!("this phone gave the app no cache folder: {error}"))?
        .join(RECEIVING_FOLDER);
    let chunk = incoming_file::chunk_of(&request)?;
    if chunk.is_first() {
        incoming_file::discard(&folder)?;
    }
    let Some(received) = incoming_file::write_chunk(&folder, &chunk)? else {
        return Ok(());
    };
    let shared = SharedFile {
        path: received.path.to_string_lossy().into_owned(),
        media_type: received.media_type,
    };
    sheet
        .0
        .run_mobile_plugin_async::<()>("presentFile", shared)
        .await
        .map_err(|error| format!("{} was not handed to the share sheet: {error}", received.path.display()))
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("share-sheet")
        .invoke_handler(tauri::generate_handler![present])
        .setup(|app, api| {
            #[cfg(target_os = "android")]
            let handle = api.register_android_plugin(ANDROID_PACKAGE, ANDROID_CLASS)?;
            #[cfg(target_os = "ios")]
            let handle = api.register_ios_plugin(init_plugin_share_sheet)?;
            app.manage(ShareSheet(handle));
            Ok(())
        })
        .build()
}
