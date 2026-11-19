use incoming_file::ReceivedFile;
use serde::Serialize;
use std::path::PathBuf;
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

const PRESENCE_SCRIPT: &str = "Object.defineProperty(window, '__INTERNKIM_SHARE_SHEET__', { value: true });";
const RECEIVING_FOLDER: &str = "share-sheet";

struct ShareSheet<R: Runtime>(PluginHandle<R>);

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SharedFile {
    path: String,
    media_type: String,
}

fn cache_folder<R: Runtime>(app: &AppHandle<R>) -> Result<PathBuf, String> {
    app.path()
        .app_cache_dir()
        .map_err(|error| format!("this phone gave the app no cache folder: {error}"))
}

async fn present_file<R: Runtime>(sheet: &ShareSheet<R>, received: ReceivedFile) -> Result<(), String> {
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

#[tauri::command]
async fn present<R: Runtime>(app: AppHandle<R>, sheet: State<'_, ShareSheet<R>>, request: Request<'_>) -> Result<(), String> {
    let folder = cache_folder(&app)?.join(RECEIVING_FOLDER);
    let chunk = incoming_file::chunk_of(&request)?;
    if chunk.is_first() {
        incoming_file::discard(&folder)?;
    }
    let Some(received) = incoming_file::write_chunk(&folder, &chunk)? else {
        return Ok(());
    };
    present_file(&sheet, received).await
}

#[tauri::command]
async fn present_download<R: Runtime>(
    app: AppHandle<R>,
    sheet: State<'_, ShareSheet<R>>,
    transfer: String,
    media_type: String,
) -> Result<(), String> {
    let downloaded = incoming_file::downloaded_file(&cache_folder(&app)?, &transfer, &media_type)?;
    present_file(&sheet, downloaded).await
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("share-sheet")
        .js_init_script(PRESENCE_SCRIPT)
        .invoke_handler(tauri::generate_handler![present, present_download])
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
