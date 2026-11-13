"""Pack a Mac app into a disk image whose window shows where to drag it.

Finder keeps a window's layout in the volume it shows, so the layout is made
by Finder itself: the image is first written writable, mounted, arranged, and
only then compressed. An image holds one background picture, with no second
one for dark mode, so the picture is light and the labels Finder draws stay
dark on it.
"""

import time
from pathlib import Path

from tauri_shipping import ShipFailure, run

BACKGROUND_DIRECTORY = ".background"
APPLICATIONS_LINK = "Applications"
WINDOW_ORIGIN = (200, 120)
WINDOW_SIZE = (660, 428)
ICON_SIZE = 112
LABEL_SIZE = 13
APPLICATION_POSITION = (180, 170)
APPLICATIONS_POSITION = (480, 170)
LAYOUT_WAIT_SECONDS = 20


def layout_script(volume: str, application: str, background: str) -> str:
    left, top = WINDOW_ORIGIN
    right, bottom = left + WINDOW_SIZE[0], top + WINDOW_SIZE[1]
    return f"""
tell application "Finder"
	tell disk "{volume}"
		open
		set current view of container window to icon view
		set toolbar visible of container window to false
		set statusbar visible of container window to false
		set the bounds of container window to {{{left}, {top}, {right}, {bottom}}}
		set viewOptions to the icon view options of container window
		set arrangement of viewOptions to not arranged
		set icon size of viewOptions to {ICON_SIZE}
		set text size of viewOptions to {LABEL_SIZE}
		set background picture of viewOptions to file "{BACKGROUND_DIRECTORY}:{background}"
		set position of item "{application}" of container window to {{{APPLICATION_POSITION[0]}, {APPLICATION_POSITION[1]}}}
		set position of item "{APPLICATIONS_LINK}" of container window to {{{APPLICATIONS_POSITION[0]}, {APPLICATIONS_POSITION[1]}}}
		update without registering applications
		close
	end tell
end tell
"""


def stage(application: Path, background: Path, scratch: Path) -> Path:
    staging = scratch / "disk-image"
    (staging / BACKGROUND_DIRECTORY).mkdir(parents=True)
    run(["ditto", str(application), str(staging / application.name)], scratch)
    run(["ditto", str(background), str(staging / BACKGROUND_DIRECTORY / background.name)], scratch)
    (staging / APPLICATIONS_LINK).symlink_to("/Applications")
    return staging


def wait_for_the_layout(volume: Path) -> None:
    deadline = time.monotonic() + LAYOUT_WAIT_SECONDS
    while not (volume / ".DS_Store").exists():
        if time.monotonic() > deadline:
            raise ShipFailure(f"Finder did not write the window layout of {volume} within {LAYOUT_WAIT_SECONDS} seconds")
        time.sleep(0.5)


def arrange_window(writable: Path, application: Path, background: Path, scratch: Path) -> None:
    volume = Path("/Volumes") / application.stem
    if volume.exists():
        raise ShipFailure(f"{volume} is already mounted, and Finder would arrange that one: eject it and run again")
    run(["hdiutil", "attach", str(writable), "-readwrite", "-noverify", "-noautoopen"], scratch)
    try:
        run(["osascript", "-e", layout_script(application.stem, application.name, background.name)], scratch)
        wait_for_the_layout(volume)
        run(["sync"], scratch)
    finally:
        run(["hdiutil", "detach", str(volume)], scratch)


def pack_disk_image(application: Path, background: Path, image: Path, scratch: Path) -> None:
    writable = scratch / "writable.dmg"
    staging = stage(application, background, scratch)
    run(["hdiutil", "create", "-volname", application.stem, "-srcfolder", str(staging), "-fs", "HFS+", "-format", "UDRW", str(writable)], scratch)
    arrange_window(writable, application, background, scratch)
    run(["hdiutil", "convert", str(writable), "-format", "UDZO", "-o", str(image)], scratch)
