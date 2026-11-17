mod folder;

use percent_encoding::percent_decode_str;
use std::path::Path;
use tauri::{
    ipc::{InvokeBody, Request},
    plugin::{Builder, TauriPlugin},
    AppHandle, Manager, Runtime,
};

const FILENAME_HEADER: &str = "filename";

fn filename_of(request: &Request<'_>) -> Result<String, String> {
    let value = request
        .headers()
        .get(FILENAME_HEADER)
        .ok_or_else(|| format!("the save request has no {FILENAME_HEADER} header"))?;
    let encoded = value
        .to_str()
        .map_err(|error| format!("the {FILENAME_HEADER} header is not plain text: {error}"))?;
    let decoded = percent_decode_str(encoded)
        .decode_utf8()
        .map_err(|error| format!("the {FILENAME_HEADER} header is not percent-encoded UTF-8: {error}"))?;
    Path::new(decoded.as_ref())
        .file_name()
        .and_then(|name| name.to_str())
        .map(str::to_owned)
        .ok_or_else(|| format!("{decoded:?} has no file name to save under"))
}

fn content_of<'request>(request: &'request Request<'_>) -> Result<&'request [u8], String> {
    match request.body() {
        InvokeBody::Raw(content) => Ok(content),
        InvokeBody::Json(_) => Err("the save request body arrived as JSON instead of raw bytes".to_owned()),
    }
}

#[tauri::command]
async fn save<R: Runtime>(app: AppHandle<R>, request: Request<'_>) -> Result<(), String> {
    let downloads = app
        .path()
        .download_dir()
        .map_err(|error| format!("this computer has no downloads folder: {error}"))?;
    folder::write_into(&downloads, &filename_of(&request)?, content_of(&request)?).map(|_| ())
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("file-save")
        .invoke_handler(tauri::generate_handler![save])
        .build()
}
