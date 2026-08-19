// The messenger names an emoji and the screen shows a character, so something
// has to hold the table that maps one to the other. It has to be the same table
// Mattermost keys on, or the names miss: Mattermost takes its names from
// emoji-datasource, where the pleased face is `star-struck`, while the GitHub
// shortcodes most emoji libraries ship call it `star_struck` and have never
// heard of `thumbsup`.

//
// The table is derived here rather than imported by the app because the package
// is over a megabyte and the app needs three fields of it. Run:
//
//   bun run scripts/build-emoji-glyphs.ts
//
// tests/unit/messenger/emoji-glyphs-current.test.ts fails when what is
// committed no longer matches what this writes.

import table from 'emoji-datasource/emoji.json';

type DatasourceEmoji = {
	short_names: string[];
	unified: string;
	skin_variations?: Record<string, { unified: string }>;
};

const toneNames: Record<string, string> = {
	'1F3FB': 'light',
	'1F3FC': 'medium_light',
	'1F3FD': 'medium',
	'1F3FE': 'medium_dark',
	'1F3FF': 'dark'
};

function glyphOf(unified: string): string {
	return String.fromCodePoint(...unified.split('-').map((point) => parseInt(point, 16)));
}

export function emojiGlyphsByName(): Map<string, string> {
	const byName = new Map<string, string>();
	for (const emoji of table as DatasourceEmoji[]) {
		for (const name of emoji.short_names) {
			byName.set(name, glyphOf(emoji.unified));
			for (const [tone, variation] of Object.entries(emoji.skin_variations ?? {})) {
				const toneName = toneNames[tone.split('-')[0]];
				if (toneName) byName.set(`${name}_${toneName}_skin_tone`, glyphOf(variation.unified));
			}
		}
	}
	return byName;
}

export function emojiGlyphsSource(): string {
	const lines = [...emojiGlyphsByName()]
		.sort(([left], [right]) => (left < right ? -1 : 1))
		.map(([name, glyph]) => `${name} ${glyph}`)
		.join('\\n');
	return `// Written by scripts/build-emoji-glyphs.ts from emoji-datasource. Do not edit.\n\nexport const emojiGlyphLines = '${lines}';\n`;
}

if (import.meta.main) {
	const path = new URL('../src/lib/messenger/emoji-glyphs.generated.ts', import.meta.url).pathname;
	await Bun.write(path, emojiGlyphsSource());
	console.log(`${emojiGlyphsByName().size} names written to ${path}`);
}
