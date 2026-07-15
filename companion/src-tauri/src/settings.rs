use std::fs;
use std::io::Write;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager};

const SETTINGS_FILENAME: &str = "companion.json";
const CURRENT_SCHEMA_VERSION: u32 = 1;

#[derive(Clone, Debug, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BackendEndpoint {
    pub base_url: String,
    pub model: String,
    #[serde(default)]
    pub embedding_model: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CompanionSettings {
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    #[serde(default)]
    pub prefer_companion_browser: bool,
    #[serde(default)]
    pub enable_local_llm: bool,
    #[serde(default = "default_backend_order")]
    pub local_backend_order: Vec<String>,
    #[serde(default = "default_ollama_endpoint")]
    pub ollama: BackendEndpoint,
    #[serde(default = "default_llamacpp_endpoint")]
    pub llamacpp: BackendEndpoint,
    #[serde(default = "default_mlx_endpoint")]
    pub mlx: BackendEndpoint,
    #[serde(default)]
    pub stt: Option<serde_json::Value>,
    #[serde(default)]
    pub tts: Option<serde_json::Value>,
}

fn default_schema_version() -> u32 {
    CURRENT_SCHEMA_VERSION
}

fn default_backend_order() -> Vec<String> {
    vec!["ollama".to_string()]
}

fn default_ollama_endpoint() -> BackendEndpoint {
    BackendEndpoint {
        base_url: "http://127.0.0.1:11434".to_string(),
        model: String::new(),
        embedding_model: None,
    }
}

fn default_llamacpp_endpoint() -> BackendEndpoint {
    BackendEndpoint {
        base_url: "http://127.0.0.1:8080".to_string(),
        model: String::new(),
        embedding_model: Some("baai/bge-m3".to_string()),
    }
}

fn default_mlx_endpoint() -> BackendEndpoint {
    BackendEndpoint {
        base_url: "http://127.0.0.1:10240".to_string(),
        model: String::new(),
        embedding_model: None,
    }
}

impl Default for CompanionSettings {
    fn default() -> Self {
        Self {
            schema_version: CURRENT_SCHEMA_VERSION,
            prefer_companion_browser: false,
            enable_local_llm: false,
            local_backend_order: default_backend_order(),
            ollama: default_ollama_endpoint(),
            llamacpp: default_llamacpp_endpoint(),
            mlx: default_mlx_endpoint(),
            stt: None,
            tts: None,
        }
    }
}

#[tauri::command]
pub fn get_settings(app: AppHandle) -> Result<CompanionSettings, String> {
    let path = settings_path(&app)?;
    Ok(read_settings(&path))
}

#[tauri::command]
pub fn set_settings(
    app: AppHandle,
    settings: CompanionSettings,
) -> Result<CompanionSettings, String> {
    validate_settings(&settings)?;
    let path = settings_path(&app)?;
    write_settings(&path, &settings)?;
    Ok(settings)
}

fn settings_path(app: &AppHandle) -> Result<PathBuf, String> {
    let directory = app
        .path()
        .app_config_dir()
        .map_err(|error| error.to_string())?;
    fs::create_dir_all(&directory).map_err(|error| error.to_string())?;
    Ok(directory.join(SETTINGS_FILENAME))
}

fn read_settings(path: &PathBuf) -> CompanionSettings {
    let document = match fs::read_to_string(path) {
        Ok(value) => value,
        Err(_) => return CompanionSettings::default(),
    };
    match serde_json::from_str::<CompanionSettings>(&document) {
        Ok(settings) => settings,
        Err(error) => {
            eprintln!("companion settings parse failed; using defaults: {error}");
            CompanionSettings::default()
        }
    }
}

fn write_settings(path: &PathBuf, settings: &CompanionSettings) -> Result<(), String> {
    let document = serde_json::to_string_pretty(settings).map_err(|error| error.to_string())?;
    let temporary_path = path.with_extension("json.tmp");
    let mut file = fs::File::create(&temporary_path).map_err(|error| error.to_string())?;
    file.write_all(document.as_bytes())
        .map_err(|error| error.to_string())?;
    file.flush().map_err(|error| error.to_string())?;
    fs::rename(&temporary_path, path).map_err(|error| error.to_string())?;
    Ok(())
}

fn validate_settings(settings: &CompanionSettings) -> Result<(), String> {
    validate_endpoint("ollama", &settings.ollama)?;
    validate_endpoint("llamacpp", &settings.llamacpp)?;
    validate_endpoint("mlx", &settings.mlx)?;
    for name in &settings.local_backend_order {
        match name.as_str() {
            "ollama" | "llamacpp" | "mlx" => {}
            other => return Err(format!("unsupported backend: {other}")),
        }
    }
    Ok(())
}

fn validate_endpoint(name: &str, endpoint: &BackendEndpoint) -> Result<(), String> {
    let trimmed_url = endpoint.base_url.trim();
    if trimmed_url.is_empty() {
        return Ok(());
    }
    if !(trimmed_url.starts_with("http://") || trimmed_url.starts_with("https://")) {
        return Err(format!(
            "{name} base URL must start with http:// or https://"
        ));
    }
    Ok(())
}
