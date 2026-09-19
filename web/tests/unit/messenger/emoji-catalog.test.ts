import { describe, expect, test } from 'bun:test';
import { catalogEmoji, emojiCategories, quickEmojiNames, searchEmoji } from '$lib/messenger/emoji-catalog';

describe('quickEmojiNames', () => {
	test('every quick emoji name resolves to a glyph in the table', () => {
		for (const name of quickEmojiNames) {
			expect(catalogEmoji(name)).toBeDefined();
		}
	});
});

describe('searchEmoji', () => {
	test('finds a known emoji by name', () => {
		const results = searchEmoji('cry', 10);
		expect(results.map((entry) => entry.glyph)).toContain('😢');
	});

	test('orders a name that starts with the query before one that merely contains it', () => {
		const results = searchEmoji('heart', 50);
		const startIndex = results.findIndex((entry) => entry.name === 'heart');
		const containsIndex = results.findIndex((entry) => entry.name === 'black_heart');
		expect(startIndex).toBeGreaterThanOrEqual(0);
		expect(containsIndex).toBeGreaterThan(startIndex);
	});

	test('never returns a skin-tone variant even when the root emoji has skin tones', () => {
		const results = searchEmoji('pray', 20);
		expect(results.some((entry) => entry.name.endsWith('_skin_tone'))).toBe(false);
		expect(results.map((entry) => entry.name)).not.toContain('pray_light_skin_tone');
	});

	test('returns nothing for an empty query', () => {
		expect(searchEmoji('', 10)).toEqual([]);
		expect(searchEmoji('   ', 10)).toEqual([]);
	});

	test('treats a hyphen and an underscore as the same separator', () => {
		const dashResult = searchEmoji('star-struck', 5);
		const underscoreResult = searchEmoji('star_struck', 5);
		expect(dashResult[0]).toEqual(underscoreResult[0]);
		expect(dashResult[0]).toEqual({ name: 'star-struck', glyph: '🤩' });
	});

	test('cuts results to the requested limit', () => {
		expect(searchEmoji('face', 3)).toHaveLength(3);
	});

	test('never repeats a glyph across results', () => {
		const results = searchEmoji('heart', 50);
		const glyphs = results.map((entry) => entry.glyph);
		expect(new Set(glyphs).size).toBe(glyphs.length);
	});
});

describe('the emoji a person browses instead of searching', () => {
	test('come in the nine groups a phone keyboard shows, faces first', () => {
		expect(emojiCategories().map((category) => category.name)).toEqual([
			'Smileys & Emotion',
			'People & Body',
			'Animals & Nature',
			'Food & Drink',
			'Travel & Places',
			'Activities',
			'Objects',
			'Symbols',
			'Flags'
		]);
	});

	test('every one of them has a character to draw, and none is a skin-tone variant', () => {
		const listed = emojiCategories().flatMap((category) => category.emoji);

		expect(listed.length).toBeGreaterThan(1800);
		expect(listed.every((emoji) => emoji.glyph.length > 0)).toBe(true);
		expect(listed.some((emoji) => emoji.name.endsWith('_skin_tone'))).toBe(false);
	});
});
