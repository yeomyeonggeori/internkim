import { describe, expect, test } from 'bun:test';
import { emojifyText, glyphOfEmojiName } from '$lib/messenger/emoji-glyph';

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

	test('reads the names Mattermost actually sends, which are not GitHub shortcodes', () => {
		expect(glyphOfEmojiName('star-struck')).toBe('🤩');
		expect(glyphOfEmojiName('thumbsup')).toBe('👍');
		expect(glyphOfEmojiName('shushing_face')).toBe('🤫');
		expect(glyphOfEmojiName('partying_face')).toBe('🥳');
	});

	test('reads either spelling, because messengers disagree about the separator', () => {
		expect(glyphOfEmojiName('star_struck')).toBe('🤩');
		expect(glyphOfEmojiName('flag_kr')).toBe(glyphOfEmojiName('flag-kr'));
		expect(glyphOfEmojiName('e_mail')).toBe(glyphOfEmojiName('e-mail'));
	});

	test('says nothing about a name it does not know, rather than guessing', () => {
		expect(glyphOfEmojiName('a_custom_company_emoji')).toBeUndefined();
		expect(glyphOfEmojiName('not_a_real_thing_light_skin_tone')).toBeUndefined();
	});
});

describe('emojifyText', () => {
	test('reads the names inside a message body', () => {
		expect(emojifyText('축하해요 :tada: :star-struck:')).toBe('축하해요 🎉 🤩');
	});

	test("leaves a name it does not know alone, because the company's own emoji look the same", () => {
		expect(emojifyText('오늘의 :company_mascot: 입니다')).toBe('오늘의 :company_mascot: 입니다');
	});

	test('leaves ordinary colons alone', () => {
		expect(emojifyText('회의: 3시')).toBe('회의: 3시');
	});
});
