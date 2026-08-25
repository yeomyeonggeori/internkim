import { expect, test } from 'bun:test';
import { fileURLToPath } from 'node:url';
import { emojiGlyphsByName, emojiGlyphsSource } from '../../../scripts/build-emoji-glyphs';

test('the committed glyph table is what the generator writes today', async () => {
	const committed = await Bun.file(
		fileURLToPath(new URL('../../../src/lib/messenger/emoji-glyphs.generated.ts', import.meta.url))
	).text();
	expect(committed).toBe(emojiGlyphsSource());
});

// glyphOfEmojiName answers a missed name in the other spelling. That is only
// safe while no two emoji in the table are separated by nothing but a hyphen
// against an underscore.
test('no two emoji differ only in how their name is separated', () => {
	const byName = emojiGlyphsByName();
	const confusable = [...byName].filter(([name, glyph]) => {
		const otherSpelling = name.includes('-') ? name.replaceAll('-', '_') : name.replaceAll('_', '-');
		const already = byName.get(otherSpelling);
		return otherSpelling !== name && already !== undefined && already !== glyph;
	});
	expect(confusable).toEqual([]);
});
