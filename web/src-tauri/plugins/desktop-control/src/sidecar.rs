use serde::Serialize;
use std::{
    path::{Path, PathBuf},
    process::{Output, Stdio},
    time::Duration,
};
use tokio::{io::AsyncWriteExt, process::Command};

#[derive(Serialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum Answer {
    Output {
        output: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        usage: Option<String>,
    },
    Refusal {
        refusal: String,
    },
}

impl Answer {
    pub fn refusal(reason: impl Into<String>) -> Self {
        Self::Refusal {
            refusal: reason.into(),
        }
    }

    fn printed(output: String) -> Self {
        Self::Output {
            output,
            usage: None,
        }
    }

    fn refuses_its_arguments(&self) -> bool {
        let Self::Output { output, .. } = self else {
            return false;
        };
        let last_line = output.trim().lines().last().unwrap_or_default();
        serde_json::from_str::<serde_json::Value>(last_line)
            .is_ok_and(|envelope| envelope["error"]["code"] == ARGUMENTS_REFUSED)
    }

    fn with_usage(self, usage: Option<String>) -> Self {
        match self {
            Self::Output { output, .. } => Self::Output { output, usage },
            refusal => refusal,
        }
    }
}

#[derive(Clone, Copy)]
pub struct Sidecar<'a> {
    pub macos_helper: Option<&'a Path>,
}

const LONGEST_QUOTED_ERROR: usize = 400;
const ARGUMENTS_REFUSED: &str = "INVALID_ARGS";
const USAGE_LIMIT: Duration = Duration::from_secs(10);

impl Sidecar<'_> {
    pub async fn run(
        self,
        program: &str,
        arguments: &[String],
        text: Option<String>,
        limit: Duration,
    ) -> Answer {
        let answer = self.run_once(program, arguments, text, limit).await;
        let Some(command) = arguments.first() else {
            return answer;
        };
        if !answer.refuses_its_arguments() {
            return answer;
        }
        let usage = self.usage_of(program, command).await;
        answer.with_usage(usage)
    }

    async fn usage_of(self, program: &str, command: &str) -> Option<String> {
        let arguments = [command.to_string(), "--help".to_string()];
        match self.run_once(program, &arguments, None, USAGE_LIMIT).await {
            Answer::Output { output, .. } => Some(output),
            Answer::Refusal { .. } => None,
        }
    }

    async fn run_once(
        self,
        program: &str,
        arguments: &[String],
        text: Option<String>,
        limit: Duration,
    ) -> Answer {
        let mut child = match self.spawn(program, arguments, text.is_some()) {
            Ok(child) => child,
            Err(error) => {
                return Answer::refusal(format!(
                    "{program} could not start on this computer: {error}"
                ))
            }
        };
        if let (Some(text), Some(mut input)) = (text, child.stdin.take()) {
            tauri::async_runtime::spawn(async move { input.write_all(text.as_bytes()).await });
        }
        match tokio::time::timeout(limit, child.wait_with_output()).await {
            Err(_) => Answer::refusal(format!(
                "{program} ran longer than {} seconds and was stopped",
                limit.as_secs()
            )),
            Ok(Err(error)) => {
                Answer::refusal(format!("{program}'s answer could not be read: {error}"))
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
        return Answer::printed(printed);
    }
    let complaint: String = String::from_utf8_lossy(&output.stderr)
        .trim()
        .chars()
        .take(LONGEST_QUOTED_ERROR)
        .collect();
    Answer::refusal(format!(
        "{program} ended with {} and printed no answer: {complaint}",
        output.status
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn printed(output: &str) -> Answer {
        Answer::printed(output.to_string())
    }

    #[test]
    fn an_envelope_refusing_the_arguments_is_told_apart_by_its_code() {
        let refused = r#"{"version":"2.6","ok":false,"command":"type","error":{"code":"INVALID_ARGS","message":"error: unexpected argument '--app' found"}}"#;
        assert!(printed(refused).refuses_its_arguments());
        assert!(printed(&format!("a notice first\n{refused}\n")).refuses_its_arguments());
    }

    #[test]
    fn any_other_answer_is_not_an_argument_refusal() {
        for output in [
            r#"{"ok":false,"command":"type","error":{"code":"POLICY_DENIED","message":"INVALID_ARGS"}}"#,
            r#"{"ok":true,"command":"snapshot","data":{"name":"INVALID_ARGS"}}"#,
            r#"{"success":false,"data":null,"error":"INVALID_ARGS"}"#,
            "Usage: agent-desktop type [OPTIONS] <REF> [TEXT]",
        ] {
            assert!(!printed(output).refuses_its_arguments(), "{output}");
        }
        assert!(!Answer::refusal("INVALID_ARGS").refuses_its_arguments());
    }

    #[test]
    fn usage_travels_beside_the_output_and_only_when_there_is_some() {
        let refused = printed("{}").with_usage(Some("Usage: type <REF>".to_string()));
        assert_eq!(
            serde_json::to_value(refused).unwrap(),
            serde_json::json!({ "output": "{}", "usage": "Usage: type <REF>" })
        );
        assert_eq!(
            serde_json::to_value(printed("{}")).unwrap(),
            serde_json::json!({ "output": "{}" })
        );
        assert_eq!(
            serde_json::to_value(Answer::refusal("off")).unwrap(),
            serde_json::json!({ "refusal": "off" })
        );
    }
}
