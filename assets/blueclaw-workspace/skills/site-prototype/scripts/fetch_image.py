#!/usr/bin/env python3
import json
import pathlib
import sys
import urllib.parse
import urllib.request

OPENVERSE_ENDPOINT = "https://api.openverse.org/v1/images/"
SAFE_LICENSES = "cc0,pdm"
MAXIMUM_BYTES = 3_500_000
USER_AGENT = "internkim-site-prototype/1.0 (prototype image sourcing)"


def search_openverse(query: str) -> list:
    parameters = urllib.parse.urlencode({
        "q": query,
        "license": SAFE_LICENSES,
        "page_size": 10,
        "aspect_ratio": "wide",
        "size": "large",
    })
    request = urllib.request.Request(OPENVERSE_ENDPOINT + "?" + parameters, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(request, timeout=20) as response:
        return json.load(response).get("results", [])


def download(url: str, output_path: pathlib.Path) -> int:
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(request, timeout=30) as response:
        data = response.read(MAXIMUM_BYTES + 1)
    if len(data) > MAXIMUM_BYTES:
        return 0
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(data)
    return len(data)


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: fetch_image.py <search query> <output path under app/public/images/>")
        return 2
    query = sys.argv[1]
    output_path = pathlib.Path(sys.argv[2])
    try:
        results = search_openverse(query)
    except Exception as error:
        print(f"image search failed: {error}; skip imagery or try a simpler English query")
        return 1
    for result in results:
        image_url = result.get("url") or ""
        if not image_url:
            continue
        try:
            written = download(image_url, output_path)
        except Exception:
            continue
        if written:
            title = result.get("title") or "untitled"
            creator = result.get("creator") or "unknown"
            print(f"saved {output_path} ({written // 1024}KB) — \"{title}\" by {creator}, license {result.get('license', '?').upper()} (no attribution required)")
            print(f"reference it as /{output_path.as_posix().split('public/', 1)[-1] if 'public/' in output_path.as_posix() else output_path.name}")
            return 0
    print(f"no usable cc0/public-domain image found for {query!r}; try a simpler English query or skip imagery")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
