#!/usr/bin/env python3
import hashlib
import os
from pathlib import Path
import re
import subprocess
import sys
import venv


BOOTSTRAP_READY_ENVIRONMENT_PREFIX = "INTERNKIM_SKILL_BOOTSTRAP_READY"
BOOTSTRAP_DISABLE_ENVIRONMENT_PREFIX = "INTERNKIM_SKILL_BOOTSTRAP_DISABLE"


def ensure_requirements(skill_name):
    requirements_path = Path(__file__).with_name("requirements.txt")
    if not requirements_path.exists() or requirements_path.read_text(encoding="utf-8").strip() == "":
        return True
    if os.environ.get(bootstrap_disable_environment_variable(skill_name)) == "1":
        return False
    if os.environ.get(bootstrap_ready_environment_variable(skill_name)) == "1":
        return True

    environment_path = dependency_environment_path(skill_name)
    python_path = environment_path / "bin" / "python"
    try:
        if not python_path.exists():
            venv.EnvBuilder(with_pip=True).create(environment_path)
        install_requirements_if_needed(python_path, requirements_path, environment_path)
    except Exception as error_value:
        sys.stderr.write(f"warning: {skill_name} dependency bootstrap failed: {error_value}\n")
        return False

    if is_current_python(python_path):
        return True

    environment = os.environ.copy()
    environment[bootstrap_ready_environment_variable(skill_name)] = "1"
    os.execve(
        str(python_path),
        [str(python_path), str(Path(sys.argv[0]).resolve()), *sys.argv[1:]],
        environment,
    )
    return False


def install_requirements_if_needed(python_path, requirements_path, environment_path):
    marker_path = environment_path / f".requirements-{requirements_hash(requirements_path)}.installed"
    if marker_path.exists():
        return
    install_requirements(python_path, requirements_path)
    marker_path.write_text("ok\n", encoding="utf-8")


def requirements_hash(requirements_path):
    return hashlib.sha256(requirements_path.read_bytes()).hexdigest()[:16]


def install_requirements(python_path, requirements_path):
    environment = os.environ.copy()
    dependency_cache = Path("/workspace/shared/cache/dependencies/pip")
    if dependency_cache.parent.exists():
        dependency_cache.mkdir(parents=True, exist_ok=True)
        environment["PIP_CACHE_DIR"] = str(dependency_cache)

    result = subprocess.run(
        [
            str(python_path),
            "-m",
            "pip",
            "install",
            "--disable-pip-version-check",
            "--quiet",
            "-r",
            str(requirements_path),
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=environment,
        check=False,
    )
    if result.returncode != 0:
        message = (result.stderr or result.stdout or "pip install failed").strip()
        raise RuntimeError(message)


def dependency_environment_path(skill_name):
    root = os.environ.get("BLUECLAW_REQUESTER_TMP")
    if root is None or root.strip() == "":
        root = str(Path.cwd())
    return Path(root) / ".skill-env" / safe_name(skill_name)


def is_current_python(python_path):
    return Path(sys.executable).resolve() == python_path.resolve()


def safe_name(value):
    normalized = re.sub(r"[^a-zA-Z0-9_.-]+", "-", value.strip()).strip("-")
    if normalized == "":
        return "default"
    return normalized.lower()


def bootstrap_ready_environment_variable(skill_name):
    return f"{BOOTSTRAP_READY_ENVIRONMENT_PREFIX}_{environment_suffix(skill_name)}"


def bootstrap_disable_environment_variable(skill_name):
    return f"{BOOTSTRAP_DISABLE_ENVIRONMENT_PREFIX}_{environment_suffix(skill_name)}"


def environment_suffix(skill_name):
    normalized = re.sub(r"[^A-Z0-9]+", "_", skill_name.upper()).strip("_")
    if normalized == "":
        return "DEFAULT"
    return normalized


def main():
    if len(sys.argv) < 3 or sys.argv[1] != "python":
        print("usage: skill_runtime.py python <script.py> [args...]", file=sys.stderr)
        sys.exit(2)
    skill_name = Path(__file__).parents[1].name
    if not ensure_requirements(skill_name):
        sys.exit(1)
    os.execv(sys.executable, [sys.executable, *sys.argv[2:]])


if __name__ == "__main__":
    main()
