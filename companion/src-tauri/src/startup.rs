use std::fs;
use std::path::{Path, PathBuf};

const LAUNCH_AGENT_LABEL: &str = "kim.intern.companion";

pub fn ensure_launch_at_login() -> Result<(), String> {
    if std::env::consts::OS != "macos" {
        return Ok(());
    }
    let app_bundle = current_app_bundle_path()?;
    let path = launch_agent_path()?;
    fs::create_dir_all(
        path.parent()
            .ok_or_else(|| "launch agent directory is unavailable".to_string())?,
    )
    .map_err(|error| error.to_string())?;
    let document = launch_agent_document(&app_bundle);
    if fs::read_to_string(&path).ok().as_deref() == Some(document.as_str()) {
        return Ok(());
    }
    fs::write(path, document).map_err(|error| error.to_string())
}

fn current_app_bundle_path() -> Result<PathBuf, String> {
    let executable = std::env::current_exe().map_err(|error| error.to_string())?;
    let contents_directory = executable
        .parent()
        .and_then(Path::parent)
        .ok_or_else(|| "app bundle path is unavailable".to_string())?;
    let app_bundle = contents_directory
        .parent()
        .ok_or_else(|| "app bundle path is unavailable".to_string())?;
    if app_bundle.extension().and_then(|value| value.to_str()) != Some("app") {
        return Err("app bundle path is unavailable".to_string());
    }
    Ok(app_bundle.to_path_buf())
}

fn launch_agent_path() -> Result<PathBuf, String> {
    let home_directory =
        std::env::var_os("HOME").ok_or_else(|| "HOME is unavailable".to_string())?;
    Ok(PathBuf::from(home_directory)
        .join("Library")
        .join("LaunchAgents")
        .join(format!("{LAUNCH_AGENT_LABEL}.plist")))
}

fn launch_agent_document(app_bundle: &Path) -> String {
    let app_bundle = escape_xml(app_bundle.to_string_lossy().as_ref());
    format!(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{LAUNCH_AGENT_LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/bin/open</string>
    <string>{app_bundle}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
</dict>
</plist>
"#
    )
}

fn escape_xml(value: &str) -> String {
    value
        .replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
        .replace('\'', "&apos;")
}
