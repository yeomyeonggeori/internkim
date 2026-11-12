mod commands;
mod consent;
mod sidecar;
mod tray;

use consent::{Consent, Language};
use serde::Serialize;
use sidecar::Answer;
use std::{
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
}

impl DesktopControl {
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
    let language = language.unwrap_or_default();
    if consent::ask(&window, language).await {
        let consent = Consent {
            is_allowed: true,
            language,
        };
        consent::write(&app, consent)?;
        apply(&app, consent);
    }
    Ok(state_of(&app).await)
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
        return Answer::Refusal(refusal);
    }
    sidecar::run(program, &arguments, text, limit).await
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
    let Answer::Output(printed) = run_if_allowed(
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
            disallow,
            run,
            open_permission_guide
        ])
        .setup(|app, _api| {
            let consent = consent::read(app);
            app.manage(DesktopControl {
                consent: Mutex::new(consent),
                turn: tokio::sync::Mutex::new(()),
            });
            show_tray(app, consent);
            Ok(())
        })
        .build()
}
