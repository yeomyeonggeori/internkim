mod commands;
mod consent;
#[cfg(target_os = "macos")]
mod macos_helper;
mod sidecar;
mod tray;

use consent::{Consent, Language};
use serde::Serialize;
use sidecar::{Answer, Sidecar};
use std::{
    path::PathBuf,
    sync::{Mutex, PoisonError},
    time::Duration,
};
use tauri::{
    plugin::{Builder, TauriPlugin},
    AppHandle, Emitter, Manager, Runtime, State, WebviewWindow,
};

const CHANGED_EVENT: &str = "desktop-control://changed";
const RUN_LIMIT: Duration = Duration::from_secs(110);
const PERMISSIONS_LIMIT: Duration = Duration::from_secs(15);

struct DesktopControl {
    consent: Mutex<Consent>,
    turn: tokio::sync::Mutex<()>,
    macos_helper: Option<PathBuf>,
}

impl DesktopControl {
    fn sidecar(&self) -> Sidecar<'_> {
        Sidecar {
            macos_helper: self.macos_helper.as_deref(),
        }
    }

    fn consent(&self) -> Consent {
        *self.consent.lock().unwrap_or_else(PoisonError::into_inner)
    }

    fn remember(&self, consent: Consent) {
        *self.consent.lock().unwrap_or_else(PoisonError::into_inner) = consent;
    }
}

#[derive(Serialize)]
struct ControlState {
    is_allowed: bool,
    permissions: Option<serde_json::Value>,
}

#[derive(Serialize, Clone)]
struct Changed {
    is_allowed: bool,
}

#[tauri::command]
async fn state<R: Runtime>(app: AppHandle<R>) -> Result<ControlState, String> {
    Ok(state_of(&app).await)
}

#[tauri::command]
async fn allow<R: Runtime>(
    app: AppHandle<R>,
    window: WebviewWindow<R>,
    language: Option<Language>,
) -> Result<ControlState, String> {
    ask_and_remember(&app, &window, language.unwrap_or_default()).await?;
    Ok(state_of(&app).await)
}

#[tauri::command]
async fn ask_once<R: Runtime>(
    app: AppHandle<R>,
    window: WebviewWindow<R>,
    language: Option<Language>,
) -> Result<ControlState, String> {
    if app.state::<DesktopControl>().consent().is_undecided() {
        ask_and_remember(&app, &window, language.unwrap_or_default()).await?;
    }
    Ok(state_of(&app).await)
}

async fn ask_and_remember<R: Runtime>(
    app: &AppHandle<R>,
    window: &WebviewWindow<R>,
    language: Language,
) -> Result<(), String> {
    let consent = Consent {
        is_allowed: consent::ask(window, language).await,
        has_been_asked: true,
        language,
    };
    consent::write(app, consent)?;
    apply(app, consent);
    Ok(())
}

#[tauri::command]
async fn disallow<R: Runtime>(app: AppHandle<R>) -> Result<ControlState, String> {
    stop_and_remember(&app)?;
    Ok(state_of(&app).await)
}

#[tauri::command]
async fn run(
    control: State<'_, DesktopControl>,
    program: Option<String>,
    arguments: Vec<String>,
    text: Option<String>,
) -> Result<Answer, String> {
    let _turn = control.turn.lock().await;
    let program = program.unwrap_or_else(|| commands::DEFAULT_PROGRAM.to_string());
    Ok(run_if_allowed(&control, &program, arguments, text, RUN_LIMIT).await)
}

#[tauri::command]
async fn open_permission_guide(control: State<'_, DesktopControl>) -> Result<Answer, String> {
    let arguments = vec!["permissions".to_string(), "--guide".to_string()];
    Ok(run_if_allowed(
        &control,
        commands::DEFAULT_PROGRAM,
        arguments,
        None,
        PERMISSIONS_LIMIT,
    )
    .await)
}

async fn run_if_allowed(
    control: &DesktopControl,
    program: &str,
    arguments: Vec<String>,
    text: Option<String>,
    limit: Duration,
) -> Answer {
    let is_allowed = control.consent().is_allowed;
    if let Some(refusal) =
        commands::refusal_of(&commands::COMMANDS, is_allowed, program, &arguments)
    {
        return Answer::refusal(refusal);
    }
    control
        .sidecar()
        .run(program, &arguments, text, limit)
        .await
}

async fn state_of<R: Runtime>(app: &AppHandle<R>) -> ControlState {
    let control = app.state::<DesktopControl>();
    let is_allowed = control.consent().is_allowed;
    let permissions = if is_allowed {
        permissions_of(&control).await
    } else {
        None
    };
    ControlState {
        is_allowed,
        permissions,
    }
}

async fn permissions_of(control: &DesktopControl) -> Option<serde_json::Value> {
    let arguments = vec!["permissions".to_string()];
    let Answer::Output {
        output: printed, ..
    } = run_if_allowed(
        control,
        commands::DEFAULT_PROGRAM,
        arguments,
        None,
        PERMISSIONS_LIMIT,
    )
    .await
    else {
        return None;
    };
    serde_json::from_str(printed.trim()).ok()
}

fn stop<R: Runtime>(app: &AppHandle<R>) {
    if let Err(error) = stop_and_remember(app) {
        eprintln!("desktop control stopped for this session but was not saved: {error}");
    }
}

fn stop_and_remember<R: Runtime>(app: &AppHandle<R>) -> Result<(), String> {
    let language = app.state::<DesktopControl>().consent().language;
    let consent = Consent {
        is_allowed: false,
        has_been_asked: true,
        language,
    };
    apply(app, consent);
    consent::write(app, consent)
}

fn apply<R: Runtime>(app: &AppHandle<R>, consent: Consent) {
    app.state::<DesktopControl>().remember(consent);
    show_tray(app, consent);
    if let Err(error) = app.emit(
        CHANGED_EVENT,
        Changed {
            is_allowed: consent.is_allowed,
        },
    ) {
        eprintln!("the window was not told desktop control changed: {error}");
    }
}

fn show_tray<R: Runtime>(app: &AppHandle<R>, consent: Consent) {
    if !consent.is_allowed {
        return tray::hide(app);
    }
    if let Err(error) = tray::show(app, consent.language) {
        eprintln!("the desktop control tray icon did not appear: {error}");
    }
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("desktop-control")
        .invoke_handler(tauri::generate_handler![
            state,
            allow,
            ask_once,
            disallow,
            run,
            open_permission_guide
        ])
        .setup(|app, _api| {
            let consent = consent::read(app);
            app.manage(DesktopControl {
                consent: Mutex::new(consent),
                turn: tokio::sync::Mutex::new(()),
                macos_helper: macos_helper_of(app),
            });
            show_tray(app, consent);
            Ok(())
        })
        .build()
}

#[cfg(target_os = "macos")]
fn macos_helper_of<R: Runtime>(app: &AppHandle<R>) -> Option<PathBuf> {
    let bundled = sidecar::application_directory()
        .ok()?
        .join(macos_helper::NAME);
    let placed = app
        .path()
        .app_local_data_dir()
        .map_err(std::io::Error::other)
        .and_then(|directory| macos_helper::place(&bundled, &directory.join("agent-desktop")));
    match placed {
        Ok(helper) => Some(helper),
        Err(error) => {
            eprintln!("the clipboard helper was not copied out of the app bundle, so agent-desktop will refuse clipboard commands: {error}");
            Some(bundled)
        }
    }
}

#[cfg(not(target_os = "macos"))]
fn macos_helper_of<R: Runtime>(_app: &AppHandle<R>) -> Option<PathBuf> {
    None
}
