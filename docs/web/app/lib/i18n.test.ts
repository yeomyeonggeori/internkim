import { describe, expect, test } from 'bun:test';
import { localeOfPath, pathInLocale } from './i18n';

describe('the locale is read from the path', () => {
	test('a language prefix names the locale', () => {
		expect(localeOfPath('/ko/architecture')).toBe('ko');
		expect(localeOfPath('/ko')).toBe('ko');
	});

	test('no prefix is the default language', () => {
		expect(localeOfPath('/architecture')).toBe('en');
		expect(localeOfPath('/')).toBe('en');
	});
});

describe('switching language keeps the page', () => {
	test('the default language carries no prefix', () => {
		expect(pathInLocale('/ko/architecture', 'en')).toBe('/architecture');
		expect(pathInLocale('/ko', 'en')).toBe('/');
	});

	test('another language gains one', () => {
		expect(pathInLocale('/architecture', 'ko')).toBe('/ko/architecture');
		expect(pathInLocale('/', 'ko')).toBe('/ko');
	});

	test('a path already in that language is unchanged', () => {
		expect(pathInLocale('/ko/api/tools', 'ko')).toBe('/ko/api/tools');
		expect(pathInLocale('/api/tools', 'en')).toBe('/api/tools');
	});
});
