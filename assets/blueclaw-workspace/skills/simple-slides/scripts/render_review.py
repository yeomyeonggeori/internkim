#!/usr/bin/env python3
import json
import pathlib
import struct
import sys
import typing
import zlib


def main() -> int:
    if len(sys.argv) != 4:
        print("Usage: render_review.py <source.md> <deck-name> <review-dir>", file=sys.stderr)
        return 2
    source_path = pathlib.Path(sys.argv[1])
    deck_name = sys.argv[2]
    review_directory_path = pathlib.Path(sys.argv[3])
    image_paths = sorted(review_directory_path.glob(deck_name + "*.png"))
    design = read_design_tokens(source_path.parent / "DESIGN.md")
    slides = [review_slide(path, design, index + 1) for index, path in enumerate(image_paths)]
    contact_sheets = write_contact_sheets(review_directory_path, deck_name, image_paths)
    report = {
        "passed": all(slide["passed"] for slide in slides) and len(slides) > 0,
        "source": source_path.name,
        "deckName": deck_name,
        "slideCount": len(slides),
        "design": design,
        "contactSheets": contact_sheets,
        "slides": slides,
    }
    write_json(review_directory_path / "slide-review.json", report)
    write_markdown(review_directory_path / "slide-review.md", report)
    print(f"  - Slide render review: {len(slides)} slides, passed={str(report['passed']).lower()}")
    return 0 if report["passed"] else 1


def read_design_tokens(path: pathlib.Path) -> dict[str, str]:
    tokens = {}
    if not path.exists():
        return tokens
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return tokens
    parts = text.split("---", 2)
    if len(parts) < 3:
        return tokens
    section = ""
    for raw_line in parts[1].splitlines():
        line = raw_line.rstrip()
        if not line.strip():
            continue
        if not line.startswith(" ") and line.endswith(":"):
            section = line[:-1].strip()
            continue
        if not section or ":" not in line:
            continue
        key, value = line.split(":", 1)
        key = key.strip()
        value = value.strip().strip('"')
        if key and value:
            tokens[section + "." + key] = value
    return tokens


def review_slide(path: pathlib.Path, design: dict[str, str], index: int) -> dict[str, object]:
    image = read_png(path)
    background = corner_background_color(image)
    content_bounds = find_content_bounds(image, background)
    density = content_density(image, background)
    margin = margin_pixels(image, design)
    checks = {
        "nonblank": content_bounds is not None,
        "safeMargin": safe_margin_passed(content_bounds, image, margin),
        "edgeOverflow": edge_overflow_passed(content_bounds, image),
        "notTooEmpty": density >= 0.006,
        "notTooDense": density <= 0.42,
    }
    warnings = slide_warnings(checks, margin, density)
    return {
        "index": index,
        "filename": path.name,
        "width": image["width"],
        "height": image["height"],
        "contentBounds": content_bounds or {},
        "contentDensity": density,
        "marginPixel": margin,
        "passed": all(checks.values()),
        "checks": checks,
        "warnings": warnings,
    }


def read_png(path: pathlib.Path) -> dict[str, object]:
    data = path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        raise ValueError(str(path) + " is not a PNG")
    offset = 8
    width = 0
    height = 0
    color_type = 0
    bit_depth = 0
    palette = []
    compressed_parts = []
    while offset < len(data):
        length = struct.unpack(">I", data[offset:offset + 4])[0]
        chunk_type = data[offset + 4:offset + 8]
        chunk_data = data[offset + 8:offset + 8 + length]
        offset += 12 + length
        if chunk_type == b"IHDR":
            width, height, bit_depth, color_type = parse_ihdr(chunk_data)
        elif chunk_type == b"PLTE":
            palette = parse_palette(chunk_data)
        elif chunk_type == b"IDAT":
            compressed_parts.append(chunk_data)
        elif chunk_type == b"IEND":
            break
    if bit_depth != 8:
        raise ValueError(str(path) + " uses unsupported PNG bit depth")
    rows = decode_png_rows(width, height, color_type, palette, b"".join(compressed_parts))
    return {"width": width, "height": height, "rows": rows}


def parse_ihdr(chunk_data: bytes) -> tuple[int, int, int, int]:
    width, height, bit_depth, color_type, _, _, _ = struct.unpack(">IIBBBBB", chunk_data)
    return width, height, bit_depth, color_type


def parse_palette(chunk_data: bytes) -> list[tuple[int, int, int, int]]:
    return [(chunk_data[index], chunk_data[index + 1], chunk_data[index + 2], 255) for index in range(0, len(chunk_data), 3)]


def decode_png_rows(width: int, height: int, color_type: int, palette: list[tuple[int, int, int, int]], compressed_data: bytes) -> list[list[tuple[int, int, int, int]]]:
    raw = zlib.decompress(compressed_data)
    bytes_per_pixel = png_bytes_per_pixel(color_type)
    stride = width * bytes_per_pixel
    rows = []
    previous = [0] * stride
    offset = 0
    for _ in range(height):
        filter_type = raw[offset]
        offset += 1
        current = list(raw[offset:offset + stride])
        offset += stride
        reconstructed = unfilter_row(filter_type, current, previous, bytes_per_pixel)
        rows.append(pixels_from_row(reconstructed, color_type, palette))
        previous = reconstructed
    return rows


def png_bytes_per_pixel(color_type: int) -> int:
    if color_type == 0:
        return 1
    if color_type == 2:
        return 3
    if color_type == 3:
        return 1
    if color_type == 4:
        return 2
    if color_type == 6:
        return 4
    raise ValueError("unsupported PNG color type")


def unfilter_row(filter_type: int, current: list[int], previous: list[int], bytes_per_pixel: int) -> list[int]:
    row = current[:]
    for index, value in enumerate(row):
        left = row[index - bytes_per_pixel] if index >= bytes_per_pixel else 0
        up = previous[index] if index < len(previous) else 0
        upper_left = previous[index - bytes_per_pixel] if index >= bytes_per_pixel and index < len(previous) else 0
        if filter_type == 1:
            row[index] = (value + left) & 255
        elif filter_type == 2:
            row[index] = (value + up) & 255
        elif filter_type == 3:
            row[index] = (value + ((left + up) // 2)) & 255
        elif filter_type == 4:
            row[index] = (value + paeth_predictor(left, up, upper_left)) & 255
        elif filter_type != 0:
            raise ValueError("unsupported PNG filter")
    return row


def paeth_predictor(left: int, up: int, upper_left: int) -> int:
    estimate = left + up - upper_left
    left_distance = abs(estimate - left)
    up_distance = abs(estimate - up)
    upper_left_distance = abs(estimate - upper_left)
    if left_distance <= up_distance and left_distance <= upper_left_distance:
        return left
    if up_distance <= upper_left_distance:
        return up
    return upper_left


def pixels_from_row(row: list[int], color_type: int, palette: list[tuple[int, int, int, int]]) -> list[tuple[int, int, int, int]]:
    pixels = []
    step = png_bytes_per_pixel(color_type)
    for index in range(0, len(row), step):
        if color_type == 0:
            value = row[index]
            pixels.append((value, value, value, 255))
        elif color_type == 2:
            pixels.append((row[index], row[index + 1], row[index + 2], 255))
        elif color_type == 3:
            pixels.append(palette[row[index]])
        elif color_type == 4:
            value = row[index]
            pixels.append((value, value, value, row[index + 1]))
        elif color_type == 6:
            pixels.append((row[index], row[index + 1], row[index + 2], row[index + 3]))
    return pixels


def corner_background_color(image: dict[str, object]) -> tuple[int, int, int, int]:
    rows = image["rows"]
    width = image["width"]
    height = image["height"]
    samples = []
    sample_size = max(6, min(width, height) // 40)
    for y in list(range(sample_size)) + list(range(height - sample_size, height)):
        for x in list(range(sample_size)) + list(range(width - sample_size, width)):
            samples.append(rows[y][x])
    return median_color(samples)


def median_color(samples: list[tuple[int, int, int, int]]) -> tuple[int, int, int, int]:
    channels = []
    for channel_index in range(4):
        values = sorted(sample[channel_index] for sample in samples)
        channels.append(values[len(values) // 2])
    return tuple(channels)


def find_content_bounds(image: dict[str, object], background: tuple[int, int, int, int]) -> typing.Optional[dict[str, int]]:
    rows = image["rows"]
    width = image["width"]
    height = image["height"]
    minimum_x = width
    minimum_y = height
    maximum_x = -1
    maximum_y = -1
    for y, row in enumerate(rows):
        for x, pixel in enumerate(row):
            if is_background_pixel(pixel, background):
                continue
            minimum_x = min(minimum_x, x)
            minimum_y = min(minimum_y, y)
            maximum_x = max(maximum_x, x)
            maximum_y = max(maximum_y, y)
    if maximum_x < 0:
        return None
    return {"left": minimum_x, "top": minimum_y, "right": maximum_x, "bottom": maximum_y}


def content_density(image: dict[str, object], background: tuple[int, int, int, int]) -> float:
    rows = image["rows"]
    content_pixels = 0
    total_pixels = image["width"] * image["height"]
    for row in rows:
        for pixel in row:
            if not is_background_pixel(pixel, background):
                content_pixels += 1
    if total_pixels == 0:
        return 0
    return round(content_pixels / total_pixels, 4)


def is_background_pixel(pixel: tuple[int, int, int, int], background: tuple[int, int, int, int]) -> bool:
    if pixel[3] < 8:
        return True
    distance = sum(abs(pixel[index] - background[index]) for index in range(3))
    return distance <= 34


def margin_pixels(image: dict[str, object], design: dict[str, str]) -> int:
    margin = parse_pixel_value(design.get("layout.margin", "68px"), 68)
    scale = image["width"] / 1280
    return max(16, round(margin * scale * 0.35))


def parse_pixel_value(value: str, fallback: int) -> int:
    cleaned = value.strip().lower().removesuffix("px")
    try:
        return int(float(cleaned))
    except ValueError:
        return fallback


def safe_margin_passed(bounds: typing.Optional[dict[str, int]], image: dict[str, object], margin: int) -> bool:
    if bounds is None:
        return False
    return bounds["left"] >= margin and bounds["top"] >= margin and image["width"] - bounds["right"] >= margin and image["height"] - bounds["bottom"] >= margin


def edge_overflow_passed(bounds: typing.Optional[dict[str, int]], image: dict[str, object]) -> bool:
    if bounds is None:
        return False
    edge = max(8, round(min(image["width"], image["height"]) * 0.015))
    return bounds["left"] > edge and bounds["top"] > edge and image["width"] - bounds["right"] > edge and image["height"] - bounds["bottom"] > edge


def slide_warnings(checks: dict[str, bool], margin: int, density: float) -> list[str]:
    warnings = []
    if not checks["nonblank"]:
        warnings.append("slide render appears blank")
    if not checks["safeMargin"]:
        warnings.append(f"content extends inside the recommended safe margin of {margin}px")
    if not checks["edgeOverflow"]:
        warnings.append("content touches the slide edge and may be clipped")
    if not checks["notTooEmpty"]:
        warnings.append(f"slide appears too sparse for a finished deck (content density {density:.1%})")
    if not checks["notTooDense"]:
        warnings.append(f"slide appears visually crowded (content density {density:.1%})")
    return warnings


def write_contact_sheets(review_directory_path: pathlib.Path, deck_name: str, image_paths: list[pathlib.Path]) -> list[dict[str, object]]:
    contact_sheets = []
    group_size = 4
    for group_index in range(0, len(image_paths), group_size):
        group_paths = image_paths[group_index:group_index + group_size]
        sheet_path = review_directory_path / f"contact-sheet-{(group_index // group_size) + 1:02d}.png"
        slide_numbers = list(range(group_index + 1, group_index + len(group_paths) + 1))
        sheet = compose_contact_sheet(group_paths, slide_numbers)
        write_png(sheet_path, sheet["width"], sheet["height"], sheet["rows"])
        contact_sheets.append({
            "filename": sheet_path.name,
            "slideNumbers": slide_numbers,
        })
    return contact_sheets


def compose_contact_sheet(image_paths: list[pathlib.Path], slide_numbers: list[int]) -> dict[str, object]:
    images = [read_png(path) for path in image_paths]
    thumbnail_width = 560
    thumbnail_height = round(thumbnail_width * images[0]["height"] / images[0]["width"]) if images else 315
    columns = 2
    rows = max(1, (len(images) + columns - 1) // columns)
    padding = 24
    gutter = 18
    label_height = 34
    sheet_width = (thumbnail_width * columns) + (gutter * (columns - 1)) + (padding * 2)
    sheet_height = ((thumbnail_height + label_height) * rows) + (gutter * (rows - 1)) + (padding * 2)
    sheet_rows = [[(248, 250, 252, 255) for _ in range(sheet_width)] for _ in range(sheet_height)]
    for index, image in enumerate(images):
        column = index % columns
        row = index // columns
        x = padding + column * (thumbnail_width + gutter)
        y = padding + row * (thumbnail_height + label_height + gutter)
        thumbnail = resize_image(image, thumbnail_width, thumbnail_height)
        draw_number_badge(sheet_rows, x, y, slide_numbers[index])
        paste_image(sheet_rows, thumbnail, x, y + label_height)
        draw_frame(sheet_rows, x, y + label_height, thumbnail_width, thumbnail_height)
    return {"width": sheet_width, "height": sheet_height, "rows": sheet_rows}


def resize_image(image: dict[str, object], target_width: int, target_height: int) -> list[list[tuple[int, int, int, int]]]:
    rows = image["rows"]
    source_width = image["width"]
    source_height = image["height"]
    resized = []
    for target_y in range(target_height):
        source_y = min(source_height - 1, round(target_y * source_height / target_height))
        source_row = rows[source_y]
        resized_row = []
        for target_x in range(target_width):
            source_x = min(source_width - 1, round(target_x * source_width / target_width))
            resized_row.append(source_row[source_x])
        resized.append(resized_row)
    return resized


def paste_image(sheet_rows: list[list[tuple[int, int, int, int]]], image_rows: list[list[tuple[int, int, int, int]]], left: int, top: int) -> None:
    for y, row in enumerate(image_rows):
        target_row = sheet_rows[top + y]
        for x, pixel in enumerate(row):
            target_row[left + x] = pixel


def draw_frame(rows: list[list[tuple[int, int, int, int]]], left: int, top: int, width: int, height: int) -> None:
    color = (203, 213, 225, 255)
    for x in range(left, left + width):
        rows[top][x] = color
        rows[top + height - 1][x] = color
    for y in range(top, top + height):
        rows[y][left] = color
        rows[y][left + width - 1] = color


def draw_number_badge(rows: list[list[tuple[int, int, int, int]]], left: int, top: int, number: int) -> None:
    digits = str(number)
    scale = 4
    digit_width = 3 * scale
    badge_width = 20 + len(digits) * digit_width + max(0, len(digits) - 1) * scale
    badge_height = 26
    fill_rect(rows, left, top, badge_width, badge_height, (17, 24, 39, 230))
    cursor = left + 10
    for digit in digits:
        draw_digit(rows, cursor, top + 5, digit, scale, (255, 255, 255, 255))
        cursor += digit_width + scale


def fill_rect(rows: list[list[tuple[int, int, int, int]]], left: int, top: int, width: int, height: int, color: tuple[int, int, int, int]) -> None:
    for y in range(top, min(len(rows), top + height)):
        row = rows[y]
        for x in range(left, min(len(row), left + width)):
            row[x] = color


def draw_digit(rows: list[list[tuple[int, int, int, int]]], left: int, top: int, digit: str, scale: int, color: tuple[int, int, int, int]) -> None:
    glyph = digit_glyphs().get(digit, digit_glyphs()["0"])
    for y, line in enumerate(glyph):
        for x, value in enumerate(line):
            if value == "1":
                fill_rect(rows, left + x * scale, top + y * scale, scale, scale, color)


def digit_glyphs() -> dict[str, list[str]]:
    return {
        "0": ["111", "101", "101", "101", "111"],
        "1": ["010", "110", "010", "010", "111"],
        "2": ["111", "001", "111", "100", "111"],
        "3": ["111", "001", "111", "001", "111"],
        "4": ["101", "101", "111", "001", "001"],
        "5": ["111", "100", "111", "001", "111"],
        "6": ["111", "100", "111", "101", "111"],
        "7": ["111", "001", "001", "001", "001"],
        "8": ["111", "101", "111", "101", "111"],
        "9": ["111", "101", "111", "001", "111"],
    }


def write_json(path: pathlib.Path, report: dict[str, object]) -> None:
    path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def write_png(path: pathlib.Path, width: int, height: int, rows: list[list[tuple[int, int, int, int]]]) -> None:
    raw_rows = []
    for row in rows:
        raw_row = bytearray([0])
        for red, green, blue, alpha in row:
            raw_row.extend([red, green, blue, alpha])
        raw_rows.append(bytes(raw_row))
    chunks = [
        png_chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)),
        png_chunk(b"IDAT", zlib.compress(b"".join(raw_rows))),
        png_chunk(b"IEND", b""),
    ]
    path.write_bytes(b"\x89PNG\r\n\x1a\n" + b"".join(chunks))


def png_chunk(chunk_type: bytes, chunk_data: bytes) -> bytes:
    checksum = zlib.crc32(chunk_type + chunk_data) & 0xFFFFFFFF
    return struct.pack(">I", len(chunk_data)) + chunk_type + chunk_data + struct.pack(">I", checksum)


def write_markdown(path: pathlib.Path, report: dict[str, object]) -> None:
    lines = [
        "# Slide Render Review",
        "",
        f"- Passed: {report['passed']}",
        f"- Slide count: {report['slideCount']}",
        f"- Contact sheets: {', '.join(sheet['filename'] for sheet in report['contactSheets'])}",
        "",
    ]
    for slide in report["slides"]:
        status = "PASS" if slide["passed"] else "WARN"
        warning_text = "; ".join(slide["warnings"]) if slide["warnings"] else "none"
        lines.append(f"## Slide {slide['index']}: {status}")
        lines.append("")
        lines.append(f"- File: {slide['filename']}")
        lines.append(f"- Content density: {slide['contentDensity']:.1%}")
        lines.append(f"- Warnings: {warning_text}")
        lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    raise SystemExit(main())
