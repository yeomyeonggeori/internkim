import { describe, expect, test } from 'bun:test';
import { glyphOfEmojiName } from '$lib/messenger/emoji-glyph';

describe('glyphOfEmojiName', () => {
	test('reads a plain name', () => {
		expect(glyphOfEmojiName('+1')).toBe('👍');
		expect(glyphOfEmojiName('tada')).toBe('🎉');
	});

	test('reads a name the messenger wrote with a skin tone', () => {
		expect(glyphOfEmojiName('+1_light_skin_tone')).toBe('👍🏻');
		expect(glyphOfEmojiName('+1_medium_dark_skin_tone')).toBe('👍🏾');
		expect(glyphOfEmojiName('raised_hands_dark_skin_tone')).toBe('🙌🏿');
	});

	test('says nothing about a name it does not know, rather than guessing', () => {
		expect(glyphOfEmojiName('a_custom_company_emoji')).toBeUndefined();
		expect(glyphOfEmojiName('not_a_real_thing_light_skin_tone')).toBeUndefined();
	});
});
