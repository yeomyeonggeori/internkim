use serde::Deserialize;
use std::{collections::BTreeMap, sync::LazyLock};

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Program {
    read: Vec<String>,
    act: Vec<String>,
    refused_options: Vec<String>,
}

#[derive(Deserialize)]
pub struct CommandSet {
    programs: BTreeMap<String, Program>,
}

pub static COMMANDS: LazyLock<CommandSet> = LazyLock::new(|| {
    serde_json::from_str(include_str!(
        "../../../../../internal/desktop/commands.json"
    ))
    .expect("internal/desktop/commands.json is not a command set")
});

pub const DEFAULT_PROGRAM: &str = "agent-desktop";

pub const NOT_ALLOWED: &str = "the person has not let internkim use this computer; they turn it on in Settings of the internkim app on this computer";

impl Program {
    fn runs(&self, command: &str) -> bool {
        self.read
            .iter()
            .chain(&self.act)
            .any(|known| known == command)
    }

    fn refuses(&self, argument: &str) -> bool {
        self.refused_options
            .iter()
            .any(|option| argument == option || argument.starts_with(&format!("{option}=")))
    }
}

pub fn refusal_of(
    commands: &CommandSet,
    is_allowed: bool,
    program_name: &str,
    arguments: &[String],
) -> Option<String> {
    if !is_allowed {
        return Some(NOT_ALLOWED.to_string());
    }
    let Some(program) = commands.programs.get(program_name) else {
        return Some(format!(
            "{program_name:?} is not a program this computer runs for internkim"
        ));
    };
    let Some(command) = arguments.first() else {
        return Some(format!(
            "arguments is empty; the first argument is the {program_name} command"
        ));
    };
    if !program.runs(command) {
        return Some(format!(
            "{command:?} is not a {program_name} command this computer runs"
        ));
    }
    let refused = arguments
        .iter()
        .find(|argument| program.refuses(argument))?;
    Some(format!(
        "this computer does not run {program_name} with {refused:?}"
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn arguments_of(words: &[&str]) -> Vec<String> {
        words.iter().map(|word| word.to_string()).collect()
    }

    fn refusal(words: &[&str]) -> Option<String> {
        refusal_of(&COMMANDS, true, DEFAULT_PROGRAM, &arguments_of(words))
    }

    fn browser_refusal(words: &[&str]) -> Option<String> {
        refusal_of(&COMMANDS, true, "agent-browser", &arguments_of(words))
    }

    #[test]
    fn refuses_everything_until_the_person_allows_it() {
        let arguments = arguments_of(&["snapshot", "--app", "Finder", "-i"]);
        assert_eq!(
            refusal_of(&COMMANDS, false, DEFAULT_PROGRAM, &arguments).as_deref(),
            Some(NOT_ALLOWED)
        );
    }

    #[test]
    fn refuses_a_program_outside_the_set() {
        let arguments = arguments_of(&["-e", "beep"]);
        let reason = refusal_of(&COMMANDS, true, "osascript", &arguments)
            .expect("an unlisted program is refused");
        assert!(reason.contains("osascript"), "{reason}");
    }

    #[test]
    fn refuses_no_command() {
        assert!(refusal(&[]).is_some_and(|reason| reason.contains("empty")));
    }

    #[test]
    fn refuses_commands_outside_the_read_and_act_lists() {
        for command in ["mcp", "batch", "session"] {
            let reason = refusal(&[command]).expect("an unlisted command is refused");
            assert!(reason.contains(command), "{reason}");
        }
        for command in ["eval", "cookies", "upload", "auth"] {
            let reason = browser_refusal(&[command]).expect("an unlisted command is refused");
            assert!(reason.contains(command), "{reason}");
        }
    }

    #[test]
    fn refuses_a_refused_option_given_apart_or_joined() {
        assert!(
            refusal(&["snapshot", "--trace", "x"]).is_some_and(|reason| reason.contains("--trace"))
        );
        assert!(
            refusal(&["snapshot", "--trace=x"]).is_some_and(|reason| reason.contains("--trace=x"))
        );
        assert!(
            browser_refusal(&["open", "https://example.com", "--proxy", "x"])
                .is_some_and(|reason| reason.contains("--proxy"))
        );
    }

    #[test]
    fn does_not_mistake_an_option_that_only_shares_a_prefix() {
        assert_eq!(refusal(&["snapshot", "--tracex"]), None);
    }

    #[test]
    fn keeps_each_program_to_its_own_commands() {
        assert!(refusal(&["connect", "9222"]).is_some());
        assert!(browser_refusal(&["launch", "chrome"]).is_some());
    }

    #[test]
    fn runs_a_read_and_an_act_of_each_program() {
        assert_eq!(refusal(&["snapshot", "--app", "Finder", "-i"]), None);
        assert_eq!(refusal(&["launch", "chrome", "--activate"]), None);
        assert_eq!(browser_refusal(&["snapshot", "-i", "--json"]), None);
        assert_eq!(
            browser_refusal(&["open", "https://example.com", "--json"]),
            None
        );
    }

    #[test]
    fn refuses_every_way_of_opening_or_joining_a_devtools_port() {
        for words in [
            &["launch", "chrome", "--cdp"][..],
            &["launch", "chrome", "--cdp=9222"],
        ] {
            assert!(refusal(words).is_some_and(|reason| reason.contains("--cdp")));
        }
        assert!(browser_refusal(&["connect", "9222", "--json"])
            .is_some_and(|reason| reason.contains("connect")));
        for option in ["--cdp", "--auto-connect", "--profile", "--state"] {
            assert!(
                browser_refusal(&["open", "https://example.com", option, "x"])
                    .is_some_and(|reason| reason.contains(option))
            );
        }
    }
}
