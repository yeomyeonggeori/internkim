use std::process::Command;

use tauri::{AppHandle, LogicalPosition, Manager, Position};

const OVERLAY_WINDOW_LABEL: &str = "browser-handoff-overlay";
const OVERLAY_WIDTH: f64 = 520.0;
const BROWSER_TOOLBAR_OFFSET: f64 = 84.0;
const BROWSER_TOP_MARGIN: f64 = 18.0;

#[derive(Clone, Copy, Debug, PartialEq)]
struct BrowserWindowBounds {
    left: f64,
    top: f64,
    width: f64,
    height: f64,
}

#[derive(Clone, Copy, Debug, PartialEq)]
struct OverlayPosition {
    left: f64,
    top: f64,
}

#[tauri::command]
pub fn sync_handoff_overlay(app: AppHandle, is_active: bool) -> Result<(), String> {
    let window = app
        .get_webview_window(OVERLAY_WINDOW_LABEL)
        .ok_or_else(|| "browser handoff overlay window is unavailable".to_string())?;
    if !is_active {
        window.hide().map_err(|error| error.to_string())?;
        return Ok(());
    }
    let Some(browser_bounds) = find_chrome_window_bounds()? else {
        window.hide().map_err(|error| error.to_string())?;
        return Ok(());
    };
    let position = overlay_position_from_bounds(browser_bounds);
    window
        .set_position(Position::Logical(LogicalPosition::new(
            position.left,
            position.top,
        )))
        .map_err(|error| error.to_string())?;
    window.show().map_err(|error| error.to_string())?;
    Ok(())
}

fn overlay_position_from_bounds(bounds: BrowserWindowBounds) -> OverlayPosition {
    let centered_left = bounds.left + ((bounds.width - OVERLAY_WIDTH) / 2.0).max(0.0);
    let top_offset = if bounds.height > 160.0 {
        BROWSER_TOOLBAR_OFFSET
    } else {
        BROWSER_TOP_MARGIN
    };
    OverlayPosition {
        left: centered_left.round(),
        top: (bounds.top + top_offset).round(),
    }
}

fn find_chrome_window_bounds() -> Result<Option<BrowserWindowBounds>, String> {
    match std::env::consts::OS {
        "macos" => find_chrome_window_bounds_macos(),
        "windows" => find_chrome_window_bounds_windows(),
        "linux" => find_chrome_window_bounds_linux(),
        _ => Err(
            "browser handoff native overlay is unsupported on this operating system".to_string(),
        ),
    }
}

fn find_chrome_window_bounds_macos() -> Result<Option<BrowserWindowBounds>, String> {
    let script = r#"
tell application "System Events"
  set frontProcessName to name of first application process whose frontmost is true
  if frontProcessName is not "Google Chrome" and frontProcessName is not "Intern Kim Companion" then return "hidden"
  set chromeProcesses to application processes whose bundle identifier is "com.google.Chrome"
  if (count of chromeProcesses) is 0 then return "hidden"
  tell first item of chromeProcesses
    if (count of windows) is 0 then return "hidden"
    set browserWindow to first window
    if value of attribute "AXMinimized" of browserWindow is true then return "hidden"
    try
      if value of attribute "AXFullScreen" of browserWindow is true then return "hidden"
    end try
    set windowPosition to position of browserWindow
    set windowSize to size of browserWindow
    return (item 1 of windowPosition as text) & "," & (item 2 of windowPosition as text) & "," & (item 1 of windowSize as text) & "," & (item 2 of windowSize as text)
  end tell
end tell
"#;
    parse_bounds_output(run_command("osascript", &["-e", script])?)
}

fn find_chrome_window_bounds_windows() -> Result<Option<BrowserWindowBounds>, String> {
    let script = r#"
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class NativeWindow {
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out Rect rect);
  [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);
}
public struct Rect { public int Left; public int Top; public int Right; public int Bottom; }
"@
Add-Type -AssemblyName System.Windows.Forms
$foregroundWindow = [NativeWindow]::GetForegroundWindow()
$foregroundProcessID = 0
[void][NativeWindow]::GetWindowThreadProcessId($foregroundWindow, [ref]$foregroundProcessID)
$foregroundProcess = Get-Process -Id $foregroundProcessID -ErrorAction SilentlyContinue
if ($foregroundProcess.ProcessName -notin @("chrome", "Intern Kim Companion")) { "hidden"; exit 0 }
$chrome = Get-Process chrome -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
if ($null -eq $chrome -or [NativeWindow]::IsIconic($chrome.MainWindowHandle)) { "hidden"; exit 0 }
$rect = New-Object Rect
[void][NativeWindow]::GetWindowRect($chrome.MainWindowHandle, [ref]$rect)
$screen = [System.Windows.Forms.Screen]::FromHandle($chrome.MainWindowHandle).Bounds
if ($rect.Left -le $screen.Left -and $rect.Top -le $screen.Top -and ($rect.Right - $rect.Left) -ge $screen.Width -and ($rect.Bottom - $rect.Top) -ge $screen.Height) { "hidden"; exit 0 }
"$($rect.Left),$($rect.Top),$($rect.Right - $rect.Left),$($rect.Bottom - $rect.Top)"
"#;
    parse_bounds_output(run_command(
        "powershell",
        &["-NoProfile", "-Command", script],
    )?)
}

fn find_chrome_window_bounds_linux() -> Result<Option<BrowserWindowBounds>, String> {
    if is_linux_wayland_session(
        std::env::var("XDG_SESSION_TYPE").unwrap_or_default(),
        std::env::var("WAYLAND_DISPLAY").unwrap_or_default(),
    ) {
        return Err("browser handoff native overlay is not supported on Linux Wayland".to_string());
    }
    let script = r#"
active_window="$(xdotool getactivewindow 2>/dev/null)" || { echo hidden; exit 0; }
window_class="$(xdotool getwindowclassname "$active_window" 2>/dev/null || true)"
case "$window_class" in
  *Chrome*|*chrome*|*Chromium*|*chromium*) ;;
  *) echo hidden; exit 0 ;;
esac
display_geometry="$(xdotool getdisplaygeometry 2>/dev/null || true)"
window_geometry="$(xdotool getwindowgeometry --shell "$active_window" 2>/dev/null)"
screen_width="$(printf '%s' "$display_geometry" | awk '{print $1}')"
screen_height="$(printf '%s' "$display_geometry" | awk '{print $2}')"
window_left="$(printf '%s\n' "$window_geometry" | awk -F= '/^X=/ {print $2}')"
window_top="$(printf '%s\n' "$window_geometry" | awk -F= '/^Y=/ {print $2}')"
window_width="$(printf '%s\n' "$window_geometry" | awk -F= '/^WIDTH=/ {print $2}')"
window_height="$(printf '%s\n' "$window_geometry" | awk -F= '/^HEIGHT=/ {print $2}')"
if [ "${window_left:-1}" -le 0 ] && [ "${window_top:-1}" -le 0 ] && [ -n "$screen_width" ] && [ -n "$screen_height" ] && [ "${window_width:-0}" -ge "$screen_width" ] && [ "${window_height:-0}" -ge "$screen_height" ]; then
  echo hidden
  exit 0
fi
xdotool getwindowgeometry --shell "$active_window" 2>/dev/null
"#;
    parse_linux_geometry_output(run_command("sh", &["-c", script])?)
}

fn is_linux_wayland_session(session_type: String, wayland_display: String) -> bool {
    session_type.trim().eq_ignore_ascii_case("wayland") || !wayland_display.trim().is_empty()
}

fn parse_bounds_output(output: String) -> Result<Option<BrowserWindowBounds>, String> {
    let trimmed_output = output.trim();
    if trimmed_output.is_empty() || trimmed_output == "hidden" {
        return Ok(None);
    }
    let values: Vec<f64> = trimmed_output
        .split(',')
        .filter_map(|value| value.trim().parse::<f64>().ok())
        .collect();
    if values.len() != 4 {
        return Err("browser window bounds are unavailable".to_string());
    }
    Ok(Some(BrowserWindowBounds {
        left: values[0],
        top: values[1],
        width: values[2],
        height: values[3],
    }))
}

fn parse_linux_geometry_output(output: String) -> Result<Option<BrowserWindowBounds>, String> {
    if output.trim() == "hidden" {
        return Ok(None);
    }
    let mut left = None;
    let mut top = None;
    let mut width = None;
    let mut height = None;
    for line in output.lines() {
        let Some((key, value)) = line.split_once('=') else {
            continue;
        };
        let parsed_value = value.trim().parse::<f64>().ok();
        match key.trim() {
            "X" => left = parsed_value,
            "Y" => top = parsed_value,
            "WIDTH" => width = parsed_value,
            "HEIGHT" => height = parsed_value,
            _ => {}
        }
    }
    match (left, top, width, height) {
        (Some(left), Some(top), Some(width), Some(height)) => Ok(Some(BrowserWindowBounds {
            left,
            top,
            width,
            height,
        })),
        _ => Err("browser window bounds are unavailable".to_string()),
    }
}

fn run_command(command: &str, arguments: &[&str]) -> Result<String, String> {
    let output = Command::new(command)
        .args(arguments)
        .output()
        .map_err(|error| error.to_string())?;
    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
    }
    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn positions_overlay_under_browser_toolbar() {
        let position = overlay_position_from_bounds(BrowserWindowBounds {
            left: 100.0,
            top: 40.0,
            width: 1200.0,
            height: 900.0,
        });

        assert_eq!(
            position,
            OverlayPosition {
                left: 440.0,
                top: 124.0
            }
        );
    }

    #[test]
    fn parses_linux_window_geometry() {
        let bounds = parse_linux_geometry_output(
            "WINDOW=123\nX=12\nY=34\nWIDTH=800\nHEIGHT=600\n".to_string(),
        )
        .unwrap()
        .unwrap();

        assert_eq!(
            bounds,
            BrowserWindowBounds {
                left: 12.0,
                top: 34.0,
                width: 800.0,
                height: 600.0
            }
        );
    }

    #[test]
    fn rejects_linux_wayland_session_values() {
        assert!(is_linux_wayland_session(
            "wayland".to_string(),
            "".to_string()
        ));
        assert!(is_linux_wayland_session(
            "x11".to_string(),
            "wayland-0".to_string()
        ));
        assert!(!is_linux_wayland_session("x11".to_string(), "".to_string()));
    }
}
