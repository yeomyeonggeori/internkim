use serde::{Deserialize, Serialize};
use std::{fs, path::PathBuf};
use tauri::{AppHandle, Manager, Runtime, WebviewWindow};
use tauri_plugin_dialog::{DialogExt, MessageDialogButtons, MessageDialogKind};

#[derive(Serialize, Deserialize, Default, Clone, Copy, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum Language {
    #[default]
    En,
    Ko,
}

#[derive(Serialize, Deserialize, Default, Clone, Copy)]
pub struct Consent {
    pub is_allowed: bool,
    #[serde(default)]
    pub has_been_asked: bool,
    #[serde(default)]
    pub language: Language,
}

impl Consent {
    pub fn is_undecided(self) -> bool {
        !self.is_allowed && !self.has_been_asked
    }
}

const CONSENT_FILE: &str = "desktop-control.json";

fn consent_path<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<PathBuf> {
    Ok(app.path().app_config_dir()?.join(CONSENT_FILE))
}

pub fn read<R: Runtime>(app: &AppHandle<R>) -> Consent {
    consent_path(app)
        .ok()
        .and_then(|path| fs::read(path).ok())
        .and_then(|bytes| serde_json::from_slice(&bytes).ok())
        .unwrap_or_default()
}

pub fn write<R: Runtime>(app: &AppHandle<R>, consent: Consent) -> Result<(), String> {
    let path = consent_path(app).map_err(|error| error.to_string())?;
    let document = serde_json::to_vec(&consent).map_err(|error| error.to_string())?;
    if let Some(directory) = path.parent() {
        fs::create_dir_all(directory)
            .map_err(|error| format!("{}: {error}", directory.display()))?;
    }
    fs::write(&path, document).map_err(|error| format!("{}: {error}", path.display()))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn consent_from(saved: &str) -> Consent {
        serde_json::from_str(saved).unwrap()
    }

    #[test]
    fn a_computer_nobody_has_answered_for_is_undecided() {
        assert!(Consent::default().is_undecided());
    }

    #[test]
    fn an_answer_either_way_is_a_decision() {
        assert!(!consent_from(r#"{"is_allowed":false,"has_been_asked":true}"#).is_undecided());
        assert!(!consent_from(r#"{"is_allowed":true,"has_been_asked":true}"#).is_undecided());
    }

    #[test]
    fn a_consent_saved_before_asking_was_recorded_still_counts() {
        assert!(!consent_from(r#"{"is_allowed":true,"language":"ko"}"#).is_undecided());
    }
}

pub async fn ask<R: Runtime>(window: &WebviewWindow<R>, language: Language) -> bool {
    let wording = Wording::of(language);
    let (answered, answer) = tokio::sync::oneshot::channel();
    window
        .dialog()
        .message(wording.message)
        .title(wording.title)
        .kind(MessageDialogKind::Warning)
        .buttons(MessageDialogButtons::OkCancelCustom(
            wording.allow.to_string(),
            wording.cancel.to_string(),
        ))
        .parent(window)
        .show(move |is_confirmed| {
            let _ = answered.send(is_confirmed);
        });
    answer.await.unwrap_or(false)
}

pub struct Wording {
    pub title: &'static str,
    pub message: &'static str,
    pub allow: &'static str,
    pub cancel: &'static str,
    pub stop: &'static str,
    pub tooltip: &'static str,
}

impl Wording {
    pub fn of(language: Language) -> Self {
        match language {
            Language::Ko => Self {
                title: "김인턴이 이 컴퓨터를 쓰게 할까요?",
                message: "internkim 앱이 열려 있는 동안 김인턴이 이 컴퓨터의 화면을 보고, 앱을 열고, 클릭하고, 입력할 수 있습니다.\n\n메뉴 막대나 작업 표시줄의 internkim 아이콘, 또는 설정에서 언제든 끌 수 있습니다.",
                allow: "허용",
                cancel: "취소",
                stop: "김인턴이 이 컴퓨터를 쓰지 않게 하기",
                tooltip: "김인턴이 이 컴퓨터를 쓸 수 있습니다",
            },
            Language::En => Self {
                title: "Let internkim use this computer?",
                message: "While the internkim app is open, internkim will be able to see this computer's screen and operate its apps: open them, click and type.\n\nYou can turn this off at any time from the internkim icon in the menu bar or taskbar, or in Settings.",
                allow: "Allow",
                cancel: "Cancel",
                stop: "Stop letting internkim use this computer",
                tooltip: "internkim can use this computer",
            },
        }
    }
}
