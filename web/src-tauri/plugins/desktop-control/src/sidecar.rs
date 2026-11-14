use serde::Serialize;
use std::{
    path::{Path, PathBuf},
    process::{Output, Stdio},
    time::Duration,
};
use tokio::{io::AsyncWriteExt, process::Command};

#[derive(Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Answer {
    Output(String),
    Refusal(String),
}

#[derive(Clone, Copy)]
pub struct Sidecar<'a> {
    pub macos_helper: Option<&'a Path>,
}

const LONGEST_QUOTED_ERROR: usize = 400;

impl Sidecar<'_> {
    pub async fn run(
        self,
        program: &str,
        arguments: &[String],
        text: Option<String>,
        limit: Duration,
    ) -> Answer {
        let mut child = match self.spawn(program, arguments, text.is_some()) {
            Ok(child) => child,
            Err(error) => {
                return Answer::Refusal(format!(
                    "{program} could not start on this computer: {error}"
                ))
            }
        };
        if let (Some(text), Some(mut input)) = (text, child.stdin.take()) {
            tauri::async_runtime::spawn(async move { input.write_all(text.as_bytes()).await });
        }
        match tokio::time::timeout(limit, child.wait_with_output()).await {
            Err(_) => Answer::Refusal(format!(
                "{program} ran longer than {} seconds and was stopped",
                limit.as_secs()
            )),
            Ok(Err(error)) => {
                Answer::Refusal(format!("{program}'s answer could not be read: {error}"))
            }
            Ok(Ok(output)) => answer_of(program, output),
        }
    }

    fn spawn(
        self,
        program: &str,
        arguments: &[String],
        takes_input: bool,
    ) -> std::io::Result<tokio::process::Child> {
        let executable = format!("{program}{}", std::env::consts::EXE_SUFFIX);
        let mut command = Command::new(application_directory()?.join(executable));
        if let Some(helper) = self.macos_helper {
            command.env("AGENT_DESKTOP_MACOS_HELPER_PATH", helper);
        }
        for variable in VARIABLES_NAMING_A_SESSION {
            command.env_remove(variable);
        }
        command.env("AGENT_BROWSER_SESSION", BROWSER_SESSION);
        command
            .args(arguments)
            .stdin(if takes_input {
                Stdio::piped()
            } else {
                Stdio::null()
            })
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .kill_on_drop(true)
            .spawn()
    }
}

const BROWSER_SESSION: &str = "internkim";
const VARIABLES_NAMING_A_SESSION: &[&str] = &[
    "AGENT_DESKTOP_SESSION",
    "AGENT_DESKTOP_AGENT_ID",
    "AGENT_BROWSER_NAMESPACE",
    "AGENT_BROWSER_CONFIG",
];

pub fn application_directory() -> std::io::Result<PathBuf> {
    let application = std::env::current_exe()?;
    let directory = application.parent().ok_or_else(|| {
        std::io::Error::new(
            std::io::ErrorKind::NotFound,
            "the app's own directory is unknown",
        )
    })?;
    Ok(directory.to_path_buf())
}

fn answer_of(program: &str, output: Output) -> Answer {
    let printed = String::from_utf8_lossy(&output.stdout).into_owned();
    if !printed.trim().is_empty() {
        return Answer::Output(printed);
    }
    let complaint: String = String::from_utf8_lossy(&output.stderr)
        .trim()
        .chars()
        .take(LONGEST_QUOTED_ERROR)
        .collect();
    Answer::Refusal(format!(
        "{program} ended with {} and printed no answer: {complaint}",
        output.status
    ))
}
