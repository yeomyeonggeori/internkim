use std::{
    fs,
    io::ErrorKind,
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

fn copy_across_volumes(received: &Path, destination: &Path) -> Result<(), String> {
    fs::copy(received, destination)
        .map_err(|error| format!("{} could not be copied to {}: {error}", received.display(), destination.display()))?;
    fs::remove_file(received).map_err(|error| format!("{} could not be removed after copying: {error}", received.display()))
}

pub fn move_into(folder: &Path, received: &Path) -> Result<PathBuf, String> {
    let filename = received
        .file_name()
        .and_then(|name| name.to_str())
        .ok_or_else(|| format!("{} has no file name to save under", received.display()))?;
    fs::create_dir_all(folder)
        .map_err(|error| format!("the folder {} could not be made: {error}", folder.display()))?;
    let destination = unused_path(folder, filename);
    match fs::rename(received, &destination) {
        Ok(()) => Ok(destination),
        Err(error) if error.kind() == ErrorKind::CrossesDevices => {
            copy_across_volumes(received, &destination).map(|()| destination)
        }
        Err(error) => Err(format!("{} could not be moved to {}: {error}", received.display(), destination.display())),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn numbers_a_name_before_its_extension() {
        assert_eq!(numbered_name("회의록 초안.pdf", 2), "회의록 초안 (2).pdf");
        assert_eq!(numbered_name("archive.tar.gz", 1), "archive.tar (1).gz");
        assert_eq!(numbered_name("README", 1), "README (1)");
    }

    #[test]
    fn moves_a_received_file_beside_one_of_the_same_name() {
        let root = std::env::temp_dir().join(format!("file-save-folder-test-{}", std::process::id()));
        let received = root.join("received").join("회의록.txt");
        let downloads = root.join("downloads");
        fs::create_dir_all(received.parent().unwrap()).unwrap();
        fs::create_dir_all(&downloads).unwrap();
        fs::write(downloads.join("회의록.txt"), b"earlier").unwrap();
        fs::write(&received, b"later").unwrap();

        let saved = move_into(&downloads, &received).unwrap();

        assert_eq!(saved, downloads.join("회의록 (1).txt"));
        assert_eq!(fs::read(&saved).unwrap(), b"later");
        assert_eq!(fs::read(downloads.join("회의록.txt")).unwrap(), b"earlier");
        assert!(!received.exists());
        fs::remove_dir_all(&root).unwrap();
    }

    #[test]
    fn copies_a_received_file_onto_another_volume_and_removes_it() {
        let root = std::env::temp_dir().join(format!("file-save-copy-test-{}", std::process::id()));
        fs::create_dir_all(&root).unwrap();
        let received = root.join("received.txt");
        let destination = root.join("saved.txt");
        fs::write(&received, b"content").unwrap();

        copy_across_volumes(&received, &destination).unwrap();

        assert_eq!(fs::read(&destination).unwrap(), b"content");
        assert!(!received.exists());
        fs::remove_dir_all(&root).unwrap();
    }
}
