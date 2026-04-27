use std::{
    collections::HashMap,
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    sync::{mpsc, Mutex},
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use tauri::{AppHandle, Emitter, Manager};

#[derive(Default)]
pub struct PromptBridgeState {
    url: Mutex<Option<String>>,
    requests: Mutex<HashMap<String, mpsc::Sender<Value>>>,
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
}

#[tauri::command]
pub fn start_shell_bridge(
    app: AppHandle,
    state: tauri::State<PromptBridgeState>,
) -> Result<String, String> {
    if let Some(existing_url) = state.url.lock().map_err(|error| error.to_string())?.clone() {
        return Ok(existing_url);
    }
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    let url = format!(
        "http://{}",
        listener.local_addr().map_err(|error| error.to_string())?
    );
    *state.url.lock().map_err(|error| error.to_string())? = Some(url.clone());
    let app_handle = app.clone();
    std::thread::spawn(move || {
        for stream in listener.incoming().flatten() {
            let app_clone = app_handle.clone();
            std::thread::spawn(move || {
                let _ = handle_prompt_stream(app_clone, stream);
            });
        }
    });
    Ok(url)
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

fn handle_prompt_stream(app: AppHandle, mut stream: TcpStream) -> Result<(), String> {
    let request = read_http_request(&mut stream)?;
    if request.method != "POST" {
        write_http_json(&mut stream, 405, json!({"error":"method not allowed"}))?;
        return Ok(());
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
    let (sender, receiver) = mpsc::channel();
    let state = app.state::<PromptBridgeState>();
    state
        .requests
        .lock()
        .map_err(|error| error.to_string())?
        .insert(request_id.clone(), sender);
    let _ = show_prompt_window(&app);
    app.emit(
        "prompt-request",
        prompt_request.with_request_id(request_id.clone()),
    )
    .map_err(|error| error.to_string())?;
    match receiver.recv_timeout(Duration::from_secs(600)) {
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
    })
}

struct HttpRequest {
    method: String,
    path: String,
    body: Vec<u8>,
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
    let content_length = lines.filter_map(parse_content_length).next().unwrap_or(0);
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
        body: buffer[body_start..buffer.len().min(body_start + content_length)].to_vec(),
    })
}

fn find_header_end(buffer: &[u8]) -> Option<usize> {
    buffer.windows(4).position(|window| window == b"\r\n\r\n")
}

fn parse_content_length(line: &str) -> Option<usize> {
    let (name, value) = line.split_once(':')?;
    if !name.eq_ignore_ascii_case("content-length") {
        return None;
    }
    value.trim().parse().ok()
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
    window.set_focus().map_err(|error| error.to_string())?;
    Ok(())
}
