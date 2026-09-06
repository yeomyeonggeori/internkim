import { describe, expect, test } from 'bun:test';
import cases from '../../../src/lib/person-call-me-cases.json';
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
	test('a korean reply language addresses the name with a spaced 님', () => {
		expect(defaultCallMe('샘플 이', 'ko')).toBe('샘플 님');
		expect(defaultCallMe('샘플 이', 'ko-KR')).toBe('샘플 님');
	});

	test('other languages keep the bare name', () => {
		expect(defaultCallMe('Sam', 'en')).toBe('Sam');
		expect(defaultCallMe('John Michael Smith', 'en')).toBe('John Michael');
		expect(defaultCallMe('샘플 이', 'ja')).toBe('샘플');
	});

	test('no name yields no address', () => {
		expect(defaultCallMe('', 'ko')).toBe('');
	});

	test('matches the shared call-me cases', () => {
		for (const testCase of cases.cases) {
			expect(defaultCallMe(testCase.recorded, testCase.locale)).toBe(testCase.callMe);
		}
	});

	test('does not treat an unrelated ko-prefixed tag as Korean', () => {
		expect(defaultCallMe('샘플 이', 'koala')).toBe('샘플');
	});
});
