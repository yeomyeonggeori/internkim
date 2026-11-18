mod folder;

use incoming_file::IncomingChunk;
use std::path::{Path, PathBuf};
use tauri::{
    ipc::Request,
    plugin::{Builder, TauriPlugin},
    AppHandle, Manager, Runtime,
};

const RECEIVING_FOLDER: &str = "file-save";

fn receiving_folder<R: Runtime>(app: &AppHandle<R>) -> Result<PathBuf, String> {
    app.path()
        .app_cache_dir()
        .map(|cache| cache.join(RECEIVING_FOLDER))
        .map_err(|error| format!("this computer gave the app no cache folder: {error}"))
}

fn discarded_with<T>(transfer_folder: &Path, outcome: Result<T, String>) -> Result<T, String> {
    match (incoming_file::discard(transfer_folder), outcome) {
        (Ok(()), outcome) => outcome,
        (Err(discarding), Ok(_)) => Err(discarding),
        (Err(discarding), Err(error)) => Err(format!("{error}; {discarding}")),
    }
}

fn receive(receiving: &Path, downloads: &Path, chunk: &IncomingChunk<'_>) -> Result<Option<PathBuf>, String> {
    let received = match incoming_file::write_chunk(receiving, chunk) {
        Ok(Some(received)) => received,
        Ok(None) => return Ok(None),
        Err(error) => return discarded_with(&chunk.transfer_folder_in(receiving), Err(error)),
    };
    let saved = folder::move_into(downloads, &received.path);
    discarded_with(&received.folder, saved).map(Some)
}

#[tauri::command]
async fn save<R: Runtime>(app: AppHandle<R>, request: Request<'_>) -> Result<(), String> {
    let downloads = app
        .path()
        .download_dir()
        .map_err(|error| format!("this computer has no downloads folder: {error}"))?;
    let chunk = incoming_file::chunk_of(&request)?;
    receive(&receiving_folder(&app)?, &downloads, &chunk).map(|_| ())
}

fn discard_unfinished_transfers<R: Runtime>(app: &AppHandle<R>) {
    let discarded = receiving_folder(app).and_then(|folder| incoming_file::discard(&folder));
    if let Err(error) = discarded {
        eprintln!("downloads left unfinished by an earlier run were not removed: {error}");
    }
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("file-save")
        .invoke_handler(tauri::generate_handler![save])
        .setup(|app, _api| {
            discard_unfinished_transfers(app);
            Ok(())
        })
        .build()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::Value;
    use std::fs;
    use tauri::{http::HeaderMap, ipc::InvokeBody};

    fn sent_headers(index: u64, is_last: bool) -> HeaderMap {
        let mut headers = HeaderMap::new();
        for (name, value) in [
            ("transfer", "0f1e2d3c4b5a69788796a5b4c3d2e1f0".to_owned()),
            ("filename", "%EB%B3%B4%EA%B3%A0%EC%84%9C.txt".to_owned()),
            ("media-type", "text%2Fplain".to_owned()),
            ("chunk", index.to_string()),
            ("last-chunk", is_last.to_string()),
        ] {
            headers.insert(name, value.parse().unwrap());
        }
        headers
    }

    #[test]
    fn saves_a_download_whose_chunks_arrived_as_number_arrays() {
        let root = std::env::temp_dir().join(format!("file-save-receive-test-{}", std::process::id()));
        let receiving = root.join("receiving");
        let downloads = root.join("downloads");
        let first_body = InvokeBody::Json(Value::from(vec![104, 105]));
        let last_body = InvokeBody::Json(Value::from(vec![33]));

        let first = incoming_file::chunk_from(&first_body, &sent_headers(0, false)).unwrap();
        assert_eq!(receive(&receiving, &downloads, &first).unwrap(), None);
        let last = incoming_file::chunk_from(&last_body, &sent_headers(1, true)).unwrap();
        let saved = receive(&receiving, &downloads, &last).unwrap().unwrap();

        assert_eq!(saved, downloads.join("보고서.txt"));
        assert_eq!(fs::read(&saved).unwrap(), b"hi!");
        assert_eq!(fs::read_dir(&receiving).unwrap().count(), 0);
        fs::remove_dir_all(&root).unwrap();
    }

    #[test]
    fn discards_a_transfer_whose_chunk_could_not_be_written() {
        let root = std::env::temp_dir().join(format!("file-save-refused-test-{}", std::process::id()));
        let receiving = root.join("receiving");
        let downloads = root.join("downloads");
        let body = InvokeBody::Json(Value::from(vec![104]));

        let orphaned = incoming_file::chunk_from(&body, &sent_headers(1, true)).unwrap();
        assert!(receive(&receiving, &downloads, &orphaned).is_err());

        assert!(!orphaned.transfer_folder_in(&receiving).exists());
        assert!(!downloads.exists());
        fs::remove_dir_all(&root).ok();
    }
}
