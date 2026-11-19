use incoming_file::ReceivedFile;
use serde::Serialize;
use std::path::PathBuf;
use tauri::{
    ipc::Request,
    plugin::{mobile::PluginInvokeError, Builder, PluginHandle, TauriPlugin},
    AppHandle, Manager, Runtime, State,
};

#[cfg(target_os = "android")]
const ANDROID_PACKAGE: &str = "app.intern.kim";
#[cfg(target_os = "android")]
const ANDROID_CLASS: &str = "MediaSavePlugin";

#[cfg(target_os = "ios")]
tauri::ios_plugin_binding!(init_plugin_media_save);

const PRESENCE_SCRIPT: &str = "Object.defineProperty(window, '__INTERNKIM_MEDIA_SAVE__', { value: true });";
const RECEIVING_FOLDER: &str = "media-save";
const PHOTO_PERMISSION_REFUSED: &str = "photoPermissionRefused";

struct Gallery<R: Runtime>(PluginHandle<R>);

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct GalleryItem {
    path: String,
    media_type: String,
}

#[derive(Serialize)]
enum RefusalReason {
    #[serde(rename = "photoPermission")]
    PhotoPermission,
    #[serde(rename = "other")]
    Other,
}

#[derive(Serialize)]
struct Refusal {
    reason: RefusalReason,
    message: String,
}

impl From<String> for Refusal {
    fn from(message: String) -> Self {
        Refusal {
            reason: RefusalReason::Other,
            message,
        }
    }
}

fn refusal_of(error: PluginInvokeError, received: &ReceivedFile) -> Refusal {
    let reason = match &error {
        PluginInvokeError::InvokeRejected(rejection) if rejection.code.as_deref() == Some(PHOTO_PERMISSION_REFUSED) => {
            RefusalReason::PhotoPermission
        }
        _ => RefusalReason::Other,
    };
    Refusal {
        reason,
        message: format!("{} was not saved into the photo gallery: {error}", received.path.display()),
    }
}

async fn save_into_gallery<R: Runtime>(gallery: &Gallery<R>, received: &ReceivedFile) -> Result<(), Refusal> {
    let item = GalleryItem {
        path: received.path.to_string_lossy().into_owned(),
        media_type: received.media_type.clone(),
    };
    gallery
        .0
        .run_mobile_plugin_async::<()>("saveIntoGallery", item)
        .await
        .map_err(|error| refusal_of(error, received))
}

async fn save_and_discard<R: Runtime>(gallery: &Gallery<R>, received: &ReceivedFile) -> Result<(), Refusal> {
    let saved = save_into_gallery(gallery, received).await;
    incoming_file::discard(&received.folder)?;
    saved
}

fn cache_folder<R: Runtime>(app: &AppHandle<R>) -> Result<PathBuf, String> {
    app.path()
        .app_cache_dir()
        .map_err(|error| format!("this phone gave the app no cache folder: {error}"))
}

#[tauri::command]
async fn save<R: Runtime>(app: AppHandle<R>, gallery: State<'_, Gallery<R>>, request: Request<'_>) -> Result<(), Refusal> {
    let folder = cache_folder(&app)?.join(RECEIVING_FOLDER);
    let chunk = incoming_file::chunk_of(&request)?;
    let Some(received) = incoming_file::write_chunk(&folder, &chunk)? else {
        return Ok(());
    };
    save_and_discard(&gallery, &received).await
}

#[tauri::command]
async fn save_download<R: Runtime>(
    app: AppHandle<R>,
    gallery: State<'_, Gallery<R>>,
    transfer: String,
    media_type: String,
) -> Result<(), Refusal> {
    let downloaded = incoming_file::downloaded_file(&cache_folder(&app)?, &transfer, &media_type)?;
    save_and_discard(&gallery, &downloaded).await
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("media-save")
        .js_init_script(PRESENCE_SCRIPT)
        .invoke_handler(tauri::generate_handler![save, save_download])
        .setup(|app, api| {
            #[cfg(target_os = "android")]
            let handle = api.register_android_plugin(ANDROID_PACKAGE, ANDROID_CLASS)?;
            #[cfg(target_os = "ios")]
            let handle = api.register_ios_plugin(init_plugin_media_save)?;
            app.manage(Gallery(handle));
            Ok(())
        })
        .build()
}
