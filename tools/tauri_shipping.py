"""What the Tauri ship tools share: a failure that names itself, a command runner,
the App Store Connect API key, and a throwaway checkout of the commit being shipped.
"""

import contextlib
import os
import subprocess
from collections.abc import Iterator
from pathlib import Path

REPOSITORY_ROOT = Path(__file__).resolve().parent.parent
TAURI_DIRECTORY = Path("web/src-tauri")
API_KEY_VARIABLES = ("APPLE_API_KEY", "APPLE_API_ISSUER", "APPLE_API_KEY_PATH")
FAILURE_OUTPUT_LINES = 20


class ShipFailure(Exception):
    pass


def say(line: str) -> None:
    print(line, flush=True)


def run(command: list[str], directory: Path, environment: dict[str, str] | None = None) -> str:
    try:
        finished = subprocess.run(command, cwd=directory, env=environment, capture_output=True, text=True)
    except FileNotFoundError as missing:
        raise ShipFailure(f"{command[0]} is not on PATH: {missing}") from missing
    if finished.returncode != 0:
        output = "\n".join((finished.stdout + finished.stderr).splitlines()[-FAILURE_OUTPUT_LINES:])
        raise ShipFailure(f"{' '.join(command[:4])} failed in {directory}:\n{output}")
    return finished.stdout


def api_key_environment(purpose: str) -> dict[str, str]:
    missing = [name for name in API_KEY_VARIABLES if not os.environ.get(name)]
    if missing:
        raise ShipFailure(f"set {', '.join(missing)}: the App Store Connect API key {purpose}")
    return dict(os.environ)


@contextlib.contextmanager
def checkout_of(reference: str, scratch: Path) -> Iterator[Path]:
    checkout = scratch / "checkout"
    run(["git", "worktree", "add", "--detach", str(checkout), reference], REPOSITORY_ROOT)
    try:
        commit = run(["git", "rev-parse", "--short", "HEAD"], checkout).strip()
        say(f"source: {reference} {commit}")
        yield checkout
    finally:
        subprocess.run(["git", "worktree", "remove", "--force", str(checkout)], cwd=REPOSITORY_ROOT, capture_output=True)
