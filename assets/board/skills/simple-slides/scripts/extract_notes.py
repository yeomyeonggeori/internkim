#!/usr/bin/env python3
"""Extract speaker notes from a Marp markdown file into a plain text file.

Notes live in HTML comments at the end of each slide. Titles are pulled from
the first h1/h2 in the slide body, or labeled "Slide N" if neither exists.

Usage: python3 extract_notes.py <input.md> <output.txt>
"""
import re
import sys


def extract(md_path: str, out_path: str) -> None:
    with open(md_path, 'r') as f:
        content = f.read()

    # Split on slide separator (a line that's just "---")
    # Marp uses --- as both frontmatter delimiter and slide separator.
    # First block of --- ... --- is frontmatter; subsequent --- are slide breaks.
    parts = re.split(r'\n---\n', content)
    slides = parts[1:]  # drop frontmatter

    sections = []
    for i, slide in enumerate(slides, 1):
        # Title: first # heading, or first <h1 ...>...</h1>, or fallback
        title_match = re.search(r'^#\s+(.+)$', slide, re.MULTILINE)
        if not title_match:
            title_match = re.search(r'<h1[^>]*>([^<]+)</h1>', slide)
        title = title_match.group(1).strip() if title_match else f"Slide {i}"

        # Last HTML comment in the slide = speaker notes
        notes_matches = re.findall(r'<!--\s*(.*?)\s*-->', slide, re.DOTALL)
        # Filter out Marp directives like "_class: lead" that shouldn't show as notes
        notes_candidates = [
            n for n in notes_matches
            if not re.match(r'^_\w+:', n.strip())
        ]
        notes = notes_candidates[-1].strip() if notes_candidates else "(no speaker notes)"

        sections.append(f"# Slide {i}: {title}\n\n{notes}\n")

    with open(out_path, 'w') as f:
        f.write(('\n' + '=' * 70 + '\n\n').join(sections))

    print(f'  - {len(slides)} slides extracted')


if __name__ == '__main__':
    if len(sys.argv) != 3:
        print("Usage: extract_notes.py <input.md> <output.txt>", file=sys.stderr)
        sys.exit(1)
    extract(sys.argv[1], sys.argv[2])
