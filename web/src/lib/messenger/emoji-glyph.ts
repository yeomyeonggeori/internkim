import { emojiGlyphLines } from './emoji-glyphs.generated';

const glyphsByName = new Map(
	emojiGlyphLines.split('\n').map((line) => {
		const separator = line.indexOf(' ');
		return [line.slice(0, separator), line.slice(separator + 1)] as const;
	})
);

const glyphEntryList = [...glyphsByName.entries()];

export function emojiEntries(): readonly (readonly [name: string, glyph: string])[] {
	return glyphEntryList;
}

// Mattermost writes `star-struck` and GitHub-descended messengers write
// `star_struck`, so a name that misses is asked again in the other spelling.
// A hyphen and an underscore never separate two different emoji in this table
// — checked against the whole of it — so the second question cannot answer with
// somebody else's emoji.
export function glyphOfEmojiName(name: string): string | undefined {
	const known = glyphsByName.get(name);
	if (known) return known;
	const otherSpelling = name.includes('-') ? name.replaceAll('-', '_') : name.replaceAll('_', '-');
	if (otherSpelling === name) return undefined;
	return glyphsByName.get(otherSpelling);
}

// A name this table does not know is left as it arrived, because the company's
// own emoji are named the same way and are drawn from their own images later.
export function emojifyText(text: string): string {
	return text.replace(/:([a-zA-Z0-9_+-]+):/g, (written, name: string) =>
		glyphOfEmojiName(name) ?? written
	);
}
