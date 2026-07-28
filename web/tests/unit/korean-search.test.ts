import { describe, expect, test } from 'bun:test';

import { koreanSearchScore, matchesKoreanSearch } from '../../src/lib/korean-search';

describe('korean search', () => {
	test('matches plain substrings regardless of case', () => {
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '캐시')).toBe(true);
		expect(matchesKoreanSearch('Flow 모바일 간격 점검', 'flow')).toBe(true);
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '캘린더')).toBe(false);
	});

	test('matches a syllable followed by a consonant', () => {
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '동ㅈ')).toBe(true);
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '동ㅎ')).toBe(false);
	});

	test('matches an incomplete trailing syllable', () => {
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '메이')).toBe(true);
		expect(matchesKoreanSearch('캘린더 원격 동기화', '캘린')).toBe(true);
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '동자')).toBe(true);
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '메아')).toBe(false);
	});

	test('matches while the next initial is still attached as a final', () => {
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '멤')).toBe(true);
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '멩')).toBe(true);
		expect(matchesKoreanSearch('메일 캐시 동작 확인', '맴')).toBe(false);
		expect(matchesKoreanSearch('캘린더 원격 동기화', '캘린더원')).toBe(false);
	});

	test('starts matching at a syllable boundary', () => {
		expect(matchesKoreanSearch('메일', 'ㅔㅇ')).toBe(false);
	});

	test('treats an empty query as a match and scores keywords', () => {
		expect(matchesKoreanSearch('메일', '   ')).toBe(true);
		expect(koreanSearchScore('mail-item', '메이', ['메일'])).toBe(1);
		expect(koreanSearchScore('mail-item', '캘린', ['메일'])).toBe(0);
	});
});
