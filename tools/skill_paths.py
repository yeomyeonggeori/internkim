import pathlib


def skill_root_paths(repository_root: pathlib.Path) -> list[pathlib.Path]:
    root_paths: list[pathlib.Path] = []
    dependency_root = repository_root / ".dependency"
    if not dependency_root.is_dir():
        return root_paths
    for dependency_path in sorted(dependency_root.iterdir()):
        if (dependency_path / "plugin.json").is_file():
            root_paths.append(dependency_path / "skills")
    return root_paths


def find_skill_directory(repository_root: pathlib.Path, skill_name: str) -> pathlib.Path:
    for root_path in skill_root_paths(repository_root):
        skill_path = root_path / skill_name
        if (skill_path / "SKILL.md").is_file():
            return skill_path
    searched = ", ".join(str(root) for root in skill_root_paths(repository_root))
    raise SystemExit(f"no skill bundle named {skill_name!r} under {searched}")

