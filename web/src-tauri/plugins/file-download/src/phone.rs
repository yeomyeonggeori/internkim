use crate::address::fetchable_address;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::path::{Path, PathBuf};
use tauri::{
    ipc::Channel,
    plugin::{Builder, PluginHandle, TauriPlugin},
    AppHandle, Manager, Runtime, State,
};

#[cfg(target_os = "android")]
const ANDROID_PACKAGE: &str = "app.intern.kim";
#[cfg(target_os = "android")]
const ANDROID_CLASS: &str = "FileDownloadPlugin";

#[cfg(target_os = "ios")]
tauri::ios_plugin_binding!(init_plugin_file_download);

const PRESENCE_SCRIPT: &str = "Object.defineProperty(window, '__INTERNKIM_FILE_DOWNLOAD__', { value: true });";

struct Downloader<R: Runtime>(PluginHandle<R>);

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct NativeDownload {
    address: String,
    destination: String,
    on_progress: Channel<Value>,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Downloaded {
    media_type: String,
}

fn cache_folder<R: Runtime>(app: &AppHandle<R>) -> Result<PathBuf, String> {
    app.path()
        .app_cache_dir()
        .map_err(|error| format!("this phone gave the app no cache folder: {error}"))
}

fn discarded_after(destination: &Path, error: String) -> String {
    match destination.parent().map(incoming_file::discard) {
        Some(Err(discarding)) => format!("{error}; {discarding}"),
        _ => error,
    }
}

#[tauri::command]
async fn download<R: Runtime>(
    app: AppHandle<R>,
    downloader: State<'_, Downloader<R>>,
    address: String,
    filename: String,
    transfer: String,
    on_progress: Channel<Value>,
) -> Result<Downloaded, String> {
    let address = fetchable_address(&address)?;
    let destination = incoming_file::download_destination(&cache_folder(&app)?, &transfer, &filename)?;
    let native = NativeDownload {
        address,
        destination: destination.to_string_lossy().into_owned(),
        on_progress,
    };
    downloader
        .0
        .run_mobile_plugin_async::<Downloaded>("download", native)
        .await
        .map_err(|error| discarded_after(&destination, format!("{} was not downloaded: {error}", destination.display())))
}

fn discard_downloads_of_an_earlier_run<R: Runtime>(app: &AppHandle<R>) {
    let discarded = cache_folder(app).and_then(|cache| incoming_file::discard(&cache.join(incoming_file::DOWNLOADS_FOLDER)));
    if let Err(error) = discarded {
        eprintln!("downloads left by an earlier run were not removed: {error}");
    }
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("file-download")
        .js_init_script(PRESENCE_SCRIPT)
        .invoke_handler(tauri::generate_handler![download])
        .setup(|app, api| {
            discard_downloads_of_an_earlier_run(app);
            #[cfg(target_os = "android")]
            let handle = api.register_android_plugin(ANDROID_PACKAGE, ANDROID_CLASS)?;
            #[cfg(target_os = "ios")]
            let handle = api.register_ios_plugin(init_plugin_file_download)?;
            app.manage(Downloader(handle));
            Ok(())
        })
        .build()
}
