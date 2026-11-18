use percent_encoding::percent_decode_str;
use serde_json::Value;
use std::{
    borrow::Cow,
    fs::{self, OpenOptions},
    io::{ErrorKind, Write},
    path::{Path, PathBuf},
};
use tauri::{
    http::HeaderMap,
    ipc::{InvokeBody, Request},
};

const TRANSFER_HEADER: &str = "transfer";
const FILENAME_HEADER: &str = "filename";
const MEDIA_TYPE_HEADER: &str = "media-type";
const CHUNK_HEADER: &str = "chunk";
const LAST_CHUNK_HEADER: &str = "last-chunk";

pub struct IncomingChunk<'request> {
    transfer: String,
    filename: String,
    media_type: String,
    index: u64,
    is_last: bool,
    content: Cow<'request, [u8]>,
}

impl IncomingChunk<'_> {
    pub fn is_first(&self) -> bool {
        self.index == 0
    }

    pub fn transfer_folder_in(&self, folder: &Path) -> PathBuf {
        folder.join(&self.transfer)
    }
}

pub struct ReceivedFile {
    pub path: PathBuf,
    pub folder: PathBuf,
    pub media_type: String,
}

fn header_of(headers: &HeaderMap, name: &str) -> Result<String, String> {
    let value = headers
        .get(name)
        .ok_or_else(|| format!("the file chunk has no {name} header"))?;
    let encoded = value
        .to_str()
        .map_err(|error| format!("the {name} header is not plain text: {error}"))?;
    percent_decode_str(encoded)
        .decode_utf8()
        .map(Cow::into_owned)
        .map_err(|error| format!("the {name} header is not percent-encoded UTF-8: {error}"))
}

fn bare_filename(named: &str) -> Result<String, String> {
    Path::new(named)
        .file_name()
        .and_then(|name| name.to_str())
        .map(str::to_owned)
        .ok_or_else(|| format!("{named:?} has no file name to keep the file under"))
}

fn transfer_of(headers: &HeaderMap) -> Result<String, String> {
    let transfer = header_of(headers, TRANSFER_HEADER)?;
    let is_identifier = !transfer.is_empty() && transfer.chars().all(|character| character.is_ascii_hexdigit() || character == '-');
    if !is_identifier {
        return Err(format!("{transfer:?} is not a transfer identifier"));
    }
    Ok(transfer)
}

fn bytes_of(numbers: &[Value]) -> Result<Vec<u8>, String> {
    numbers
        .iter()
        .map(|number| {
            number
                .as_u64()
                .and_then(|value| u8::try_from(value).ok())
                .ok_or_else(|| format!("{number} in the file chunk is not a byte"))
        })
        .collect()
}

fn content_of(body: &InvokeBody) -> Result<Cow<'_, [u8]>, String> {
    match body {
        InvokeBody::Raw(content) => Ok(Cow::Borrowed(content)),
        InvokeBody::Json(Value::Array(numbers)) => bytes_of(numbers).map(Cow::Owned),
        InvokeBody::Json(other) => Err(format!("the file chunk arrived as {other} instead of bytes")),
    }
}

pub fn chunk_of<'request>(request: &'request Request<'_>) -> Result<IncomingChunk<'request>, String> {
    chunk_from(request.body(), request.headers())
}

pub fn chunk_from<'body>(body: &'body InvokeBody, headers: &HeaderMap) -> Result<IncomingChunk<'body>, String> {
    let index = header_of(headers, CHUNK_HEADER)?;
    Ok(IncomingChunk {
        transfer: transfer_of(headers)?,
        filename: bare_filename(&header_of(headers, FILENAME_HEADER)?)?,
        media_type: header_of(headers, MEDIA_TYPE_HEADER)?,
        index: index
            .parse()
            .map_err(|error| format!("the {CHUNK_HEADER} header {index:?} is not a chunk number: {error}"))?,
        is_last: header_of(headers, LAST_CHUNK_HEADER)? == "true",
        content: content_of(body)?,
    })
}

pub fn write_chunk(folder: &Path, chunk: &IncomingChunk<'_>) -> Result<Option<ReceivedFile>, String> {
    let transfer_folder = chunk.transfer_folder_in(folder);
    let destination = transfer_folder.join(&chunk.filename);
    if chunk.is_first() {
        fs::create_dir_all(&transfer_folder)
            .map_err(|error| format!("the folder {} could not be made: {error}", transfer_folder.display()))?;
    }
    let mut file = OpenOptions::new()
        .create(chunk.is_first())
        .truncate(chunk.is_first())
        .append(!chunk.is_first())
        .write(true)
        .open(&destination)
        .map_err(|error| format!("{} could not be opened for chunk {}: {error}", destination.display(), chunk.index))?;
    file.write_all(&chunk.content)
        .map_err(|error| format!("chunk {} of {} could not be written: {error}", chunk.index, destination.display()))?;
    if !chunk.is_last {
        return Ok(None);
    }
    Ok(Some(ReceivedFile {
        path: destination,
        folder: transfer_folder,
        media_type: chunk.media_type.clone(),
    }))
}

pub fn discard(folder: &Path) -> Result<(), String> {
    match fs::remove_dir_all(folder) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == ErrorKind::NotFound => Ok(()),
        Err(error) => Err(format!("{} could not be removed: {error}", folder.display())),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn chunk(index: u64, is_last: bool, content: &[u8]) -> IncomingChunk<'_> {
        IncomingChunk {
            transfer: "6f1c2b3a-0000-4000-8000-000000000001".to_owned(),
            filename: "회의 사진.jpg".to_owned(),
            media_type: "image/jpeg".to_owned(),
            index,
            is_last,
            content: Cow::Borrowed(content),
        }
    }

    #[test]
    fn joins_the_chunks_of_one_transfer_into_one_file() {
        let folder = std::env::temp_dir().join(format!("incoming-file-test-{}", std::process::id()));
        assert!(write_chunk(&folder, &chunk(0, false, b"first ")).unwrap().is_none());
        let received = write_chunk(&folder, &chunk(1, true, b"second")).unwrap().unwrap();
        assert_eq!(fs::read(&received.path).unwrap(), b"first second");
        assert_eq!(received.path.file_name().unwrap(), "회의 사진.jpg");
        assert_eq!(received.media_type, "image/jpeg");
        discard(&folder).unwrap();
        assert!(!folder.exists());
        discard(&folder).unwrap();
    }

    fn sent_headers(index: u64, is_last: bool) -> HeaderMap {
        let mut headers = HeaderMap::new();
        for (name, value) in [
            (TRANSFER_HEADER, "6f1c2b3a00004000800000000000000a".to_owned()),
            (FILENAME_HEADER, "%ED%9A%8C%EC%9D%98%EB%A1%9D.txt".to_owned()),
            (MEDIA_TYPE_HEADER, "text%2Fplain".to_owned()),
            (CHUNK_HEADER, index.to_string()),
            (LAST_CHUNK_HEADER, is_last.to_string()),
        ] {
            headers.insert(name, value.parse().unwrap());
        }
        headers
    }

    #[test]
    fn writes_a_file_whose_chunks_arrived_as_number_arrays() {
        let folder = std::env::temp_dir().join(format!("incoming-file-json-test-{}", std::process::id()));
        let first_body = InvokeBody::Json(Value::from(vec![104, 105]));
        let last_body = InvokeBody::Json(Value::from(vec![33]));
        let first = chunk_from(&first_body, &sent_headers(0, false)).unwrap();
        assert!(write_chunk(&folder, &first).unwrap().is_none());
        let last = chunk_from(&last_body, &sent_headers(1, true)).unwrap();
        let received = write_chunk(&folder, &last).unwrap().unwrap();
        assert_eq!(fs::read(&received.path).unwrap(), b"hi!");
        assert_eq!(received.path.file_name().unwrap(), "회의록.txt");
        assert_eq!(received.media_type, "text/plain");
        discard(&folder).unwrap();
    }

    #[test]
    fn writes_a_file_whose_chunk_arrived_as_raw_bytes() {
        let folder = std::env::temp_dir().join(format!("incoming-file-raw-test-{}", std::process::id()));
        let body = InvokeBody::Raw(b"hi".to_vec());
        let only = chunk_from(&body, &sent_headers(0, true)).unwrap();
        let received = write_chunk(&folder, &only).unwrap().unwrap();
        assert_eq!(fs::read(&received.path).unwrap(), b"hi");
        discard(&folder).unwrap();
    }

    #[test]
    fn refuses_a_body_that_is_not_bytes() {
        let body = InvokeBody::Json(Value::from("hi"));
        assert!(chunk_from(&body, &sent_headers(0, true)).is_err());
    }

    #[test]
    fn reads_the_number_array_a_webview_without_raw_requests_sends() {
        let numbers = vec![Value::from(0), Value::from(255)];
        assert_eq!(bytes_of(&numbers).unwrap(), vec![0, 255]);
        assert!(bytes_of(&[Value::from(256)]).is_err());
    }

    #[test]
    fn keeps_only_the_last_part_of_a_named_path() {
        assert_eq!(bare_filename("../../보고서.pdf").unwrap(), "보고서.pdf");
        assert!(bare_filename("..").is_err());
    }
}
