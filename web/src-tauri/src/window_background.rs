use tauri::{window::Color, App, Manager, Runtime, Theme, WebviewWindow, Window, WindowEvent};

pub const LIGHT_BACKGROUND: Color = Color(0xFF, 0xFF, 0xFF, 0xFF);
pub const DARK_BACKGROUND: Color = Color(0x09, 0x09, 0x0B, 0xFF);

fn background_of(theme: Theme) -> Color {
    match theme {
        Theme::Dark => DARK_BACKGROUND,
        _ => LIGHT_BACKGROUND,
    }
}

fn paint<R: Runtime>(window: &WebviewWindow<R>, theme: Theme) -> tauri::Result<()> {
    window.set_background_color(Some(background_of(theme)))
}

pub fn paint_every_window<R: Runtime>(app: &mut App<R>) -> Result<(), Box<dyn std::error::Error>> {
    for window in app.webview_windows().values() {
        paint(window, window.theme()?)?;
    }
    Ok(())
}

pub fn repaint_on_appearance_change<R: Runtime>(window: &Window<R>, event: &WindowEvent) {
    let WindowEvent::ThemeChanged(theme) = event else {
        return;
    };
    let Some(webview_window) = window.app_handle().get_webview_window(window.label()) else {
        return;
    };
    if let Err(error) = paint(&webview_window, *theme) {
        eprintln!(
            "the {} window kept its old background after the appearance changed: {error}",
            window.label()
        );
    }
}
