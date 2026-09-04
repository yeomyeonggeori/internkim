import { describe, expect, test } from 'bun:test';
import { defaultCallMe, languageAutonym, replyLanguageOptions, replyLanguageTags } from '../../../src/lib/persona/languages';

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

describe('default call-me', () => {
	test('a korean reply language addresses the name with 님', () => {
		expect(defaultCallMe('이샘플', 'ko')).toBe('이샘플님');
		expect(defaultCallMe('이샘플', 'ko-KR')).toBe('이샘플님');
	});

	test('other languages keep the bare name', () => {
		expect(defaultCallMe('Sam', 'en')).toBe('Sam');
		expect(defaultCallMe('이샘플', 'ja')).toBe('이샘플');
	});

	test('no name yields no address', () => {
		expect(defaultCallMe('', 'ko')).toBe('');
	});
});
