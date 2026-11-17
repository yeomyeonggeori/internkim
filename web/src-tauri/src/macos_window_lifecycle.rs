use tauri::{AppHandle, Manager, RunEvent, Runtime, Window, WindowEvent};

const MAIN_WINDOW_LABEL: &str = "main";

pub fn hide_main_window_instead_of_closing<R: Runtime>(window: &Window<R>, event: &WindowEvent) {
    let WindowEvent::CloseRequested { api, .. } = event else {
        return;
    };
    if window.label() != MAIN_WINDOW_LABEL {
        return;
    }
    api.prevent_close();
    if let Err(error) = window.hide() {
        eprintln!("the main window could not be hidden on close: {error}");
    }
}

pub fn show_main_window_on_reopen<R: Runtime>(app: &AppHandle<R>, event: RunEvent) {
    let RunEvent::Reopen {
        has_visible_windows: false,
        ..
    } = event
    else {
        return;
    };
    let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) else {
        return;
    };
    if let Err(error) = window.show().and_then(|()| window.set_focus()) {
        eprintln!("the main window could not be shown again from the dock: {error}");
    }
}
