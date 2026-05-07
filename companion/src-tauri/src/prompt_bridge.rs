use std::{
    collections::HashMap,
    fs::File,
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    sync::{mpsc, Mutex},
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use tauri::{AppHandle, Emitter, Manager};
use tauri_plugin_dialog::{DialogExt, MessageDialogButtons, MessageDialogKind};

#[derive(Default)]
pub struct PromptBridgeState {
    url: Mutex<Option<String>>,
    token: Mutex<Option<String>>,
    requests: Mutex<HashMap<String, mpsc::Sender<Value>>>,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ShellBridgeInfo {
    url: String,
    token: String,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct PromptRequest {
    request_id: String,
    kind: String,
    message: String,
    default: Option<bool>,
    tool_name: Option<String>,
    capability_scope: Option<String>,
    resource_scope: Option<Value>,
    timeout_seconds: Option<u64>,
}

#[derive(Deserialize)]
struct PromptInput {
    message: Option<String>,
    default: Option<bool>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ApprovalInput {
    tool_name: Option<String>,
    capability_scope: Option<String>,
    resource_scope: Option<Value>,
    timeout_seconds: Option<u64>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct FilePickInput {
    title: Option<String>,
    allowed_extensions: Option<Vec<String>>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct DirectoryPickInput {
    title: Option<String>,
}

#[tauri::command]
pub fn start_shell_bridge(
    app: AppHandle,
    state: tauri::State<PromptBridgeState>,
) -> Result<ShellBridgeInfo, String> {
    if let Some(existing_url) = state.url.lock().map_err(|error| error.to_string())?.clone() {
        let existing_token = state
            .token
            .lock()
            .map_err(|error| error.to_string())?
            .clone()
            .ok_or_else(|| "shell bridge token is unavailable".to_string())?;
        return Ok(ShellBridgeInfo {
            url: existing_url,
            token: existing_token,
        });
    }
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    let url = format!(
        "http://{}",
        listener.local_addr().map_err(|error| error.to_string())?
    );
    let token = new_bridge_token();
    *state.url.lock().map_err(|error| error.to_string())? = Some(url.clone());
    *state.token.lock().map_err(|error| error.to_string())? = Some(token.clone());
    let app_handle = app.clone();
    let bridge_token = token.clone();
    std::thread::spawn(move || {
        for stream in listener.incoming().flatten() {
            let app_clone = app_handle.clone();
            let token_clone = bridge_token.clone();
            std::thread::spawn(move || {
                let _ = handle_prompt_stream(app_clone, stream, token_clone);
            });
        }
    });
    Ok(ShellBridgeInfo { url, token })
}

#[tauri::command]
pub fn complete_prompt_request(
    state: tauri::State<PromptBridgeState>,
    request_id: String,
    response: Value,
) -> Result<(), String> {
    let sender = state
        .requests
        .lock()
        .map_err(|error| error.to_string())?
        .remove(&request_id)
        .ok_or_else(|| "prompt request is no longer pending".to_string())?;
    sender.send(response).map_err(|error| error.to_string())
}

fn handle_prompt_stream(
    app: AppHandle,
    mut stream: TcpStream,
    bridge_token: String,
) -> Result<(), String> {
    let request = read_http_request(&mut stream)?;
    if request.header("x-internkim-shell-bridge-token") != Some(bridge_token.as_str()) {
        write_http_json(
            &mut stream,
            403,
            json!({"error":"shell bridge token required"}),
        )?;
        return Ok(());
    }
    if request.method != "POST" {
        write_http_json(&mut stream, 405, json!({"error":"method not allowed"}))?;
        return Ok(());
    }
    if request.path == "/v1/file/pick" {
        return handle_file_pick(app, stream, request.body);
    }
    if request.path == "/v1/directory/pick" {
        return handle_directory_pick(app, stream, request.body);
    }
    let kind = match request.path.as_str() {
        "/v1/user/confirm" => "confirm",
        "/v1/user/input" => "input",
        "/v1/security/approval" => "approval",
        _ => {
            write_http_json(&mut stream, 404, json!({"error":"not found"}))?;
            return Ok(());
        }
    };
    let request_id = new_request_id();
    let prompt_request = prompt_request_from_body(kind, request.body, request_id.clone())?;
    if prompt_request.kind == "approval" {
        return handle_boolean_prompt(app, stream, prompt_request, "Allow", "Deny", "allowed");
    }
    if prompt_request.kind == "confirm" {
        return handle_boolean_prompt(app, stream, prompt_request, "Approve", "Deny", "confirmed");
    }
    let (sender, receiver) = mpsc::channel();
    let state = app.state::<PromptBridgeState>();
    state
        .requests
        .lock()
        .map_err(|error| error.to_string())?
        .insert(request_id.clone(), sender);
    let timeout = prompt_timeout(&prompt_request);
    let _ = show_prompt_window(&app);
    app.emit(
        "prompt-request",
        prompt_request.with_request_id(request_id.clone()),
    )
    .map_err(|error| error.to_string())?;
    match receiver.recv_timeout(timeout) {
        Ok(response) => write_http_json(&mut stream, 200, response)?,
        Err(_) => {
            state
                .requests
                .lock()
                .map_err(|error| error.to_string())?
                .remove(&request_id);
            write_http_json(
                &mut stream,
                408,
                json!({"error":"prompt request timed out"}),
            )?;
        }
    }
    Ok(())
}

fn handle_boolean_prompt(
    app: AppHandle,
    mut stream: TcpStream,
    prompt_request: PromptRequest,
    approve_label: &str,
    deny_label: &str,
    response_key: &str,
) -> Result<(), String> {
    let _ = show_prompt_window(&app);
    let is_approved = app
        .dialog()
        .message(prompt_request.message)
        .title("Intern Kim Companion")
        .buttons(MessageDialogButtons::OkCancelCustom(
            approve_label.to_string(),
            deny_label.to_string(),
        ))
        .kind(MessageDialogKind::Warning)
        .blocking_show();
    write_http_json(&mut stream, 200, json!({response_key:is_approved}))
}

#[tauri::command]
pub fn pick_mount_directory(app: AppHandle) -> Result<Option<String>, String> {
    let _ = show_prompt_window(&app);
    match app.dialog().file().blocking_pick_folder() {
        Some(file_path) => match file_path.into_path() {
            Ok(path) => Ok(Some(path.to_string_lossy().to_string())),
            Err(_) => Err("selected directory path is unavailable".to_string()),
        },
        None => Ok(None),
    }
}

fn handle_file_pick(app: AppHandle, mut stream: TcpStream, body: Vec<u8>) -> Result<(), String> {
    let input: FilePickInput = serde_json::from_slice(&body).map_err(|error| error.to_string())?;
    let _ = show_prompt_window(&app);
    let mut file_dialog = app.dialog().file();
    if let Some(title) = input.title.filter(|value| !value.trim().is_empty()) {
        file_dialog = file_dialog.set_title(title);
    }
    let extensions = normalize_extensions(input.allowed_extensions.unwrap_or_default());
    let extension_refs: Vec<&str> = extensions.iter().map(String::as_str).collect();
    if !extension_refs.is_empty() {
        file_dialog = file_dialog.add_filter("Allowed files", &extension_refs);
    }
    match file_dialog.blocking_pick_file() {
        Some(file_path) => match file_path.into_path() {
            Ok(path) => write_http_json(
                &mut stream,
                200,
                json!({"path": path.to_string_lossy().to_string()}),
            )?,
            Err(_) => write_http_json(
                &mut stream,
                500,
                json!({"error":"selected file path is unavailable"}),
            )?,
        },
        None => write_http_json(&mut stream, 200, json!({"cancelled":true}))?,
    }
    Ok(())
}

fn handle_directory_pick(
    app: AppHandle,
    mut stream: TcpStream,
    body: Vec<u8>,
) -> Result<(), String> {
    let input: DirectoryPickInput =
        serde_json::from_slice(&body).map_err(|error| error.to_string())?;
    let _ = show_prompt_window(&app);
    let mut file_dialog = app.dialog().file();
    if let Some(title) = input.title.filter(|value| !value.trim().is_empty()) {
        file_dialog = file_dialog.set_title(title);
    }
    match file_dialog.blocking_pick_folder() {
        Some(file_path) => match file_path.into_path() {
            Ok(path) => write_http_json(
                &mut stream,
                200,
                json!({"path": path.to_string_lossy().to_string()}),
            )?,
            Err(_) => write_http_json(
                &mut stream,
                500,
                json!({"error":"selected directory path is unavailable"}),
            )?,
        },
        None => write_http_json(&mut stream, 200, json!({"cancelled":true}))?,
    }
    Ok(())
}

impl PromptRequest {
    fn with_request_id(mut self, request_id: String) -> Self {
        self.request_id = request_id;
        self
    }
}

fn prompt_request_from_body(
    kind: &str,
    body: Vec<u8>,
    request_id: String,
) -> Result<PromptRequest, String> {
    if kind == "approval" {
        let input: ApprovalInput =
            serde_json::from_slice(&body).map_err(|error| error.to_string())?;
        let resource_value = input
            .resource_scope
            .as_ref()
            .and_then(|value| value.get("value"))
            .and_then(Value::as_str)
            .unwrap_or("this resource");
        return Ok(PromptRequest {
            request_id,
            kind: kind.to_string(),
            message: format!(
                "Allow {} work on {} for this task?",
                input
                    .capability_scope
                    .clone()
                    .unwrap_or_else(|| "companion".to_string()),
                resource_value
            ),
            default: None,
            tool_name: input.tool_name,
            capability_scope: input.capability_scope,
            resource_scope: input.resource_scope,
            timeout_seconds: input.timeout_seconds,
        });
    }
    let input: PromptInput = serde_json::from_slice(&body).map_err(|error| error.to_string())?;
    Ok(PromptRequest {
        request_id,
        kind: kind.to_string(),
        message: input.message.unwrap_or_else(|| "Continue?".to_string()),
        default: input.default,
        tool_name: None,
        capability_scope: None,
        resource_scope: None,
        timeout_seconds: None,
    })
}

fn prompt_timeout(prompt_request: &PromptRequest) -> Duration {
    let timeout_seconds = prompt_request
        .timeout_seconds
        .filter(|value| *value > 0)
        .unwrap_or(600);
    Duration::from_secs(timeout_seconds.clamp(1, 600))
}

struct HttpRequest {
    method: String,
    path: String,
    headers: HashMap<String, String>,
    body: Vec<u8>,
}

impl HttpRequest {
    fn header(&self, name: &str) -> Option<&str> {
        self.headers
            .get(&name.to_ascii_lowercase())
            .map(String::as_str)
    }
}

fn read_http_request(stream: &mut TcpStream) -> Result<HttpRequest, String> {
    let mut buffer = Vec::new();
    let mut temporary = [0_u8; 1024];
    loop {
        let count = stream
            .read(&mut temporary)
            .map_err(|error| error.to_string())?;
        if count == 0 {
            break;
        }
        buffer.extend_from_slice(&temporary[..count]);
        if find_header_end(&buffer).is_some() {
            break;
        }
        if buffer.len() > 64 * 1024 {
            return Err("request header is too large".to_string());
        }
    }
    let header_end =
        find_header_end(&buffer).ok_or_else(|| "request header is incomplete".to_string())?;
    let header_text = String::from_utf8_lossy(&buffer[..header_end]);
    let mut lines = header_text.lines();
    let first_line = lines
        .next()
        .ok_or_else(|| "request line is missing".to_string())?;
    let mut first_line_parts = first_line.split_whitespace();
    let method = first_line_parts.next().unwrap_or("").to_string();
    let path = first_line_parts.next().unwrap_or("").to_string();
    let headers = parse_headers(lines.collect());
    let content_length = headers
        .get("content-length")
        .and_then(|value| value.parse().ok())
        .unwrap_or(0);
    let body_start = header_end + 4;
    while buffer.len() < body_start + content_length {
        let count = stream
            .read(&mut temporary)
            .map_err(|error| error.to_string())?;
        if count == 0 {
            break;
        }
        buffer.extend_from_slice(&temporary[..count]);
    }
    Ok(HttpRequest {
        method,
        path,
        headers,
        body: buffer[body_start..buffer.len().min(body_start + content_length)].to_vec(),
    })
}

fn find_header_end(buffer: &[u8]) -> Option<usize> {
    buffer.windows(4).position(|window| window == b"\r\n\r\n")
}

fn parse_headers(lines: Vec<&str>) -> HashMap<String, String> {
    lines
        .into_iter()
        .filter_map(|line| {
            let (name, value) = line.split_once(':')?;
            Some((name.trim().to_ascii_lowercase(), value.trim().to_string()))
        })
        .collect()
}

fn normalize_extensions(values: Vec<String>) -> Vec<String> {
    values
        .into_iter()
        .map(|value| value.trim().trim_start_matches('.').to_lowercase())
        .filter(|value| !value.is_empty())
        .collect()
}

fn write_http_json(
    stream: &mut TcpStream,
    status_code: u16,
    document: Value,
) -> Result<(), String> {
    let body = serde_json::to_vec(&document).map_err(|error| error.to_string())?;
    let status_text = match status_code {
        200 => "OK",
        404 => "Not Found",
        405 => "Method Not Allowed",
        403 => "Forbidden",
        408 => "Request Timeout",
        _ => "Error",
    };
    let header = format!(
        "HTTP/1.1 {} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        status_code,
        status_text,
        body.len()
    );
    stream
        .write_all(header.as_bytes())
        .and_then(|_| stream.write_all(&body))
        .map_err(|error| error.to_string())
}

fn new_bridge_token() -> String {
    let mut bytes = [0_u8; 32];
    if File::open("/dev/urandom")
        .and_then(|mut file| file.read_exact(&mut bytes))
        .is_ok()
    {
        return bytes.iter().map(|byte| format!("{:02x}", byte)).collect();
    }
    let nanoseconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    format!("bridge-{}-{}", std::process::id(), nanoseconds)
}

fn new_request_id() -> String {
    let nanoseconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    format!("prompt-{}", nanoseconds)
}

fn show_prompt_window(app: &AppHandle) -> Result<(), String> {
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "main window is unavailable".to_string())?;
    window.show().map_err(|error| error.to_string())?;
    window.unminimize().map_err(|error| error.to_string())?;
    window.set_focus().map_err(|error| error.to_string())?;
    let _ = window.request_user_attention(Some(tauri::UserAttentionType::Critical));
    Ok(())
}
