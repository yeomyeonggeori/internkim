import { describe, expect, test } from 'bun:test';
import { languageAutonym, replyLanguageOptions, replyLanguageTags } from '../../../src/lib/persona/languages';

describe('reply languages', () => {
	test('each language names itself in its own script', () => {
		expect(languageAutonym('ko')).toBe('한국어');
		expect(languageAutonym('en')).toBe('English');
		expect(languageAutonym('ja')).toBe('日本語');
	});

	test('an unknown tag falls back to itself', () => {
		expect(languageAutonym('zz')).toBe('zz');
	});

	test('a selected tag outside the list leads the options', () => {
		const options = replyLanguageOptions('fi');
		expect(options[0]?.value).toBe('fi');
		expect(options).toHaveLength(replyLanguageTags.length + 1);
	});

	test('a listed selection adds nothing', () => {
		expect(replyLanguageOptions('ko')).toHaveLength(replyLanguageTags.length);
	});
});
