import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { quickEmojiGlyphs, recentEmojiGlyphs, rememberEmojiGlyph } from '$lib/messenger/recent-emoji';

const stored = new Map<string, string>();
const originalWindow = globalThis.window;

function standIn(): void {
	Object.defineProperty(globalThis, 'window', {
		value: {
			localStorage: {
				getItem: (key: string) => stored.get(key) ?? null,
				setItem: (key: string, value: string) => void stored.set(key, value),
				removeItem: (key: string) => void stored.delete(key)
			}
		},
		configurable: true
	});
}

beforeEach(() => {
	stored.clear();
	standIn();
});

afterEach(() => {
	Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
});

describe('recentEmojiGlyphs', () => {
	test('reads nothing when nothing was ever stored', () => {
		expect(recentEmojiGlyphs()).toEqual([]);
	});

	test('reads a corrupt stored value as empty', () => {
		stored.set('internkim.messenger.recent-emoji', '{not json');
		expect(recentEmojiGlyphs()).toEqual([]);
	});

	test('reads a wrongly shaped stored value as empty', () => {
		stored.set('internkim.messenger.recent-emoji', JSON.stringify({ not: 'an array' }));
		expect(recentEmojiGlyphs()).toEqual([]);
		stored.set('internkim.messenger.recent-emoji', JSON.stringify([1, 2, 3]));
		expect(recentEmojiGlyphs()).toEqual([]);
	});
});

describe('rememberEmojiGlyph', () => {
	test('puts the remembered glyph first', () => {
		rememberEmojiGlyph('😢');
		rememberEmojiGlyph('🎉');
		expect(recentEmojiGlyphs()).toEqual(['🎉', '😢']);
	});

	test('moves an already-remembered glyph to the front instead of duplicating it', () => {
		rememberEmojiGlyph('😢');
		rememberEmojiGlyph('🎉');
		rememberEmojiGlyph('😢');
		expect(recentEmojiGlyphs()).toEqual(['😢', '🎉']);
	});

	test('keeps at most 8 glyphs', () => {
		for (const glyph of ['1', '2', '3', '4', '5', '6', '7', '8', '9']) {
			rememberEmojiGlyph(glyph);
		}
		expect(recentEmojiGlyphs()).toEqual(['9', '8', '7', '6', '5', '4', '3', '2']);
	});
});

describe('quickEmojiGlyphs', () => {
	test('returns the first quick glyphs when nothing was ever remembered', () => {
		expect(quickEmojiGlyphs(3)).toEqual(['👍', '❤️', '😂']);
	});

	test('puts a remembered glyph first', () => {
		rememberEmojiGlyph('🔥');
		expect(quickEmojiGlyphs(3)[0]).toBe('🔥');
	});
});
