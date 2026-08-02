mod handoff_overlay;
mod settings;
mod startup;

use std::process::Command;

use tauri::{
    image::Image,
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    AppHandle, Emitter, Manager,
};

const TRAY_ICON_BYTES: &[u8] = include_bytes!("../icons/tray-icon.png");

#[tauri::command]
fn show_main_window(app: AppHandle) -> Result<(), String> {
    show_window(&app)
}

#[tauri::command]
fn open_admin_url(device_url: String) -> Result<(), String> {
    let trimmed_url = device_url.trim();
    if !(trimmed_url.starts_with("https://") || trimmed_url.starts_with("http://")) {
        return Err("device URL must use http or https".to_string());
    }
    let target_url = format!("{}/_internkim/admin", trimmed_url.trim_end_matches('/'));
    open_url(&target_url)
}

#[tauri::command]
fn ensure_launch_at_login() -> Result<(), String> {
    startup::ensure_launch_at_login()
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_deep_link::init())
        .setup(|app| {
            if let Err(error) = build_tray(app.handle()) {
                eprintln!("failed to build tray icon: {error}");
                let _ = show_window(app.handle());
            }
            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .invoke_handler(tauri::generate_handler![
            show_main_window,
            handoff_overlay::sync_handoff_overlay,
            open_admin_url,
            ensure_launch_at_login,
            settings::get_settings,
            settings::set_settings
        ])
        .run(tauri::generate_context!())
        .expect("error while running internkim");
}

fn build_tray(app: &AppHandle) -> tauri::Result<()> {
    let tray_icon = Image::from_bytes(TRAY_ICON_BYTES)?;
    let status_item = MenuItem::with_id(app, "status", "Status", true, None::<&str>)?;
    let connect_item = MenuItem::with_id(app, "connect", "Connect", true, None::<&str>)?;
    let admin_item = MenuItem::with_id(
        app,
        "open_admin",
        "Open internkim Admin",
        true,
        None::<&str>,
    )?;
    let quit_item = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&status_item, &connect_item, &admin_item, &quit_item])?;

    TrayIconBuilder::new()
        .icon(tray_icon)
        .tooltip("internkim")
        .menu(&menu)
        .show_menu_on_left_click(true)
        .on_menu_event(|app, event| match event.id.as_ref() {
            "status" | "connect" => {
                let _ = show_window(app);
            }
            "open_admin" => {
                let _ = show_window(app);
                let _ = app.emit("open-admin-request", ());
            }
            "quit" => app.exit(0),
            _ => {}
        })
        .build(app)?;
    Ok(())
}

fn show_window(app: &AppHandle) -> Result<(), String> {
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "main window is unavailable".to_string())?;
    window.show().map_err(|error| error.to_string())?;
    window.set_focus().map_err(|error| error.to_string())?;
    Ok(())
}

fn open_url(target_url: &str) -> Result<(), String> {
    let (command, arguments) = match std::env::consts::OS {
        "macos" => ("open", vec![target_url.to_string()]),
        "windows" => (
            "rundll32",
            vec![
                "url.dll,FileProtocolHandler".to_string(),
                target_url.to_string(),
            ],
        ),
        _ => ("xdg-open", vec![target_url.to_string()]),
    };
    Command::new(command)
        .args(arguments)
        .spawn()
        .map(|_| ())
        .map_err(|error| error.to_string())
}
