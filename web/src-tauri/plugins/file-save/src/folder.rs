use std::{
    fs,
    path::{Path, PathBuf},
};

fn numbered_name(filename: &str, number: u32) -> String {
    let path = Path::new(filename);
    let stem = path.file_stem().and_then(|stem| stem.to_str()).unwrap_or(filename);
    match path.extension().and_then(|extension| extension.to_str()) {
        Some(extension) => format!("{stem} ({number}).{extension}"),
        None => format!("{stem} ({number})"),
    }
}

fn unused_path(folder: &Path, filename: &str) -> PathBuf {
    let mut candidate = folder.join(filename);
    let mut number = 1;
    while candidate.exists() {
        candidate = folder.join(numbered_name(filename, number));
        number += 1;
    }
    candidate
}

pub fn write_into(folder: &Path, filename: &str, content: &[u8]) -> Result<PathBuf, String> {
    fs::create_dir_all(folder)
        .map_err(|error| format!("the folder {} could not be made: {error}", folder.display()))?;
    let destination = unused_path(folder, filename);
    fs::write(&destination, content)
        .map_err(|error| format!("{} could not be written: {error}", destination.display()))?;
    Ok(destination)
}

#[cfg(test)]
mod tests {
    use super::numbered_name;

    #[test]
    fn numbers_a_name_before_its_extension() {
        assert_eq!(numbered_name("회의록 초안.pdf", 2), "회의록 초안 (2).pdf");
        assert_eq!(numbered_name("archive.tar.gz", 1), "archive.tar (1).gz");
        assert_eq!(numbered_name("README", 1), "README (1)");
    }
}
