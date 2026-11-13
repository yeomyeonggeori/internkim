use std::{env, fs, path::Path, process};

fn main() {
    refuse_a_build_missing_its_sidecars();
    tauri_build::build()
}

fn refuse_a_build_missing_its_sidecars() {
    let target = env::var("TARGET").unwrap_or_default();
    let missing: Vec<String> = sidecars_of_this_platform()
        .into_iter()
        .map(|sidecar| sidecar_path(&sidecar, &target))
        .filter(|path| !Path::new(path).exists())
        .collect();
    if missing.is_empty() {
        return;
    }
    eprintln!(
        "the desktop app bundles programs that are not here yet: {}\nrun `tools/prepare-agent-desktop --target {target}` from the repository root, then build again",
        missing.join(", ")
    );
    process::exit(1);
}

fn sidecars_of_this_platform() -> Vec<String> {
    let platform = env::var("CARGO_CFG_TARGET_OS").unwrap_or_default();
    let configuration_path = format!("tauri.{platform}.conf.json");
    println!("cargo:rerun-if-changed={configuration_path}");
    let Ok(document) = fs::read_to_string(&configuration_path) else {
        return Vec::new();
    };
    let configuration: serde_json::Value = serde_json::from_str(&document)
        .unwrap_or_else(|error| panic!("{configuration_path} is not json: {error}"));
    configuration["bundle"]["externalBin"]
        .as_array()
        .map(|sidecars| {
            sidecars
                .iter()
                .filter_map(|sidecar| sidecar.as_str().map(str::to_string))
                .collect()
        })
        .unwrap_or_default()
}

fn sidecar_path(sidecar: &str, target: &str) -> String {
    let extension = if target.contains("windows") { ".exe" } else { "" };
    format!("{sidecar}-{target}{extension}")
}
