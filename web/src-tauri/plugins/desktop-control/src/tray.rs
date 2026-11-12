use crate::consent::{Language, Wording};
use tauri::{
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    AppHandle, Runtime,
};

const TRAY_ID: &str = "desktop-control";
const STOP_ITEM_ID: &str = "desktop-control-stop";

pub fn show<R: Runtime>(app: &AppHandle<R>, language: Language) -> tauri::Result<()> {
    if app.tray_by_id(TRAY_ID).is_some() {
        return Ok(());
    }
    let wording = Wording::of(language);
    let stop = MenuItem::with_id(app, STOP_ITEM_ID, wording.stop, true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&stop])?;
    let builder = TrayIconBuilder::with_id(TRAY_ID)
        .menu(&menu)
        .tooltip(wording.tooltip)
        .on_menu_event(|app, event| {
            if event.id() == STOP_ITEM_ID {
                crate::stop(app);
            }
        });
    let builder = match app.default_window_icon() {
        Some(icon) => builder.icon(icon.clone()),
        None => builder,
    };
    builder.build(app)?;
    Ok(())
}

pub fn hide<R: Runtime>(app: &AppHandle<R>) {
    app.remove_tray_by_id(TRAY_ID);
}
