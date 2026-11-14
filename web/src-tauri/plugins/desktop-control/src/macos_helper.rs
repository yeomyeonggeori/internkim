use std::{
    fs,
    io::{self, Write},
    os::unix::fs::{DirBuilderExt, OpenOptionsExt},
    path::{Path, PathBuf},
};

pub const NAME: &str = "agent-desktop-macos-helper";

pub fn place(bundled: &Path, directory: &Path) -> io::Result<PathBuf> {
    let placed = directory.join(NAME);
    let wanted = fs::read(bundled)?;
    if fs::read(&placed).is_ok_and(|held| held == wanted) {
        return Ok(placed);
    }
    fs::DirBuilder::new()
        .recursive(true)
        .mode(0o700)
        .create(directory)?;
    let staged = directory.join(format!(".{NAME}.{}", std::process::id()));
    remove_if_present(&staged)?;
    fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o755)
        .open(&staged)?
        .write_all(&wanted)?;
    fs::rename(&staged, &placed)?;
    Ok(placed)
}

fn remove_if_present(path: &Path) -> io::Result<()> {
    match fs::remove_file(path) {
        Err(error) if error.kind() != io::ErrorKind::NotFound => Err(error),
        _ => Ok(()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::os::unix::fs::{MetadataExt, PermissionsExt};

    struct Scratch(PathBuf);

    impl Scratch {
        fn new(name: &str) -> Self {
            let path =
                std::env::temp_dir().join(format!("desktop-control-{name}-{}", std::process::id()));
            let _ = fs::remove_dir_all(&path);
            fs::create_dir_all(&path).unwrap();
            Self(path)
        }

        fn bundled(&self, contents: &[u8]) -> PathBuf {
            let path = self.0.join("bundle").join(NAME);
            fs::create_dir_all(path.parent().unwrap()).unwrap();
            fs::write(&path, contents).unwrap();
            path
        }
    }

    impl Drop for Scratch {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    #[test]
    fn places_an_identical_single_link_copy_only_its_owner_can_change() {
        let scratch = Scratch::new("identical");
        let bundled = scratch.bundled(b"helper build one");
        let directory = scratch.0.join("support").join("agent-desktop");

        let placed = place(&bundled, &directory).unwrap();

        assert_eq!(placed, directory.join(NAME));
        assert_eq!(fs::read(&placed).unwrap(), b"helper build one");
        let file = fs::symlink_metadata(&placed).unwrap();
        assert!(file.file_type().is_file());
        assert_eq!(file.nlink(), 1);
        assert_eq!(file.permissions().mode() & 0o777, 0o755);
        let folder = fs::metadata(&directory).unwrap();
        assert_eq!(folder.permissions().mode() & 0o777, 0o700);
        assert_eq!(fs::read_dir(&directory).unwrap().count(), 1);
    }

    #[test]
    fn leaves_a_copy_that_already_matches_where_it_is() {
        let scratch = Scratch::new("matching");
        let bundled = scratch.bundled(b"helper build one");
        let directory = scratch.0.join("agent-desktop");
        let first = fs::metadata(place(&bundled, &directory).unwrap()).unwrap();

        let second = fs::metadata(place(&bundled, &directory).unwrap()).unwrap();

        assert_eq!(first.ino(), second.ino());
    }

    #[test]
    fn replaces_a_copy_of_another_build() {
        let scratch = Scratch::new("replaced");
        let directory = scratch.0.join("agent-desktop");
        place(&scratch.bundled(b"helper build one"), &directory).unwrap();

        let placed = place(&scratch.bundled(b"helper build two"), &directory).unwrap();

        assert_eq!(fs::read(placed).unwrap(), b"helper build two");
        assert_eq!(fs::read_dir(&directory).unwrap().count(), 1);
    }
}
