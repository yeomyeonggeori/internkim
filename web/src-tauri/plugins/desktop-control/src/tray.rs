use crate::consent::{Language, Wording};
use tauri::{
    image::Image,
    include_image,
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    AppHandle, Runtime,
};

const TRAY_ID: &str = "desktop-control";
const STOP_ITEM_ID: &str = "desktop-control-stop";
const MENU_BAR_ICON: Image<'_> = include_image!("icons/menu-bar.png");

pub fn show<R: Runtime>(app: &AppHandle<R>, language: Language) -> tauri::Result<()> {
    if app.tray_by_id(TRAY_ID).is_some() {
        return Ok(());
    }
    let wording = Wording::of(language);
    let stop = MenuItem::with_id(app, STOP_ITEM_ID, wording.stop, true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&stop])?;
    TrayIconBuilder::with_id(TRAY_ID)
        .menu(&menu)
        .tooltip(wording.tooltip)
        .icon(MENU_BAR_ICON)
        .icon_as_template(true)
        .on_menu_event(|app, event| {
            if event.id() == STOP_ITEM_ID {
                crate::stop(app);
            }
        })
        .build(app)?;
    Ok(())
}

pub fn hide<R: Runtime>(app: &AppHandle<R>) {
    app.remove_tray_by_id(TRAY_ID);
}
