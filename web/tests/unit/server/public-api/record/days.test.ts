import { describe, expect, test } from 'bun:test';
import { dayIn, instantWritten, weekWindow } from '$lib/server/public-api/record/days';
import { labelOf, LabelUnresolved, labelsOfVocabulary } from '$lib/server/public-api/record/labels';

describe('the day a moment falls on', () => {
	test('is the company’s day, not the caller’s', () => {
		const lateInSeoul = new Date('2026-08-31T16:00:00.000Z');
		expect(dayIn('Asia/Seoul', lateInSeoul)).toBe('2026-09-01');
		expect(dayIn('UTC', lateInSeoul)).toBe('2026-08-31');
	});
});

describe('a window of weeks', () => {
	const wednesday = new Date('2026-08-26T03:00:00.000Z');

	test('runs monday to sunday of the week asked for', () => {
		expect(weekWindow('Asia/Seoul', wednesday, 0, 0)).toEqual({ from: '2026-08-24', to: '2026-08-30' });
	});

	test('spans from the earlier week to the later one', () => {
		expect(weekWindow('Asia/Seoul', wednesday, -1, 1)).toEqual({ from: '2026-08-17', to: '2026-09-06' });
	});

	test('reads a reversed pair the way it was meant', () => {
		expect(weekWindow('Asia/Seoul', wednesday, 1, -1)).toEqual(weekWindow('Asia/Seoul', wednesday, -1, 1));
	});
});

describe('a date a caller wrote', () => {
	test('is a whole day where the company is, not where the caller is', () => {
		expect(instantWritten('Asia/Seoul', '2026-08-31')).toBe('2026-08-30T15:00:00.000Z');
		expect(instantWritten('Asia/Seoul', '2026-08-31', true)).toBe('2026-08-31T14:59:59.999Z');
		expect(instantWritten('UTC', '2026-08-31')).toBe('2026-08-31T00:00:00.000Z');
	});

	test('reads back as the day it named, in that company’s clock', () => {
		expect(dayIn('Asia/Seoul', new Date(instantWritten('Asia/Seoul', '2026-08-31', true)))).toBe('2026-08-31');
		expect(dayIn('America/New_York', new Date(instantWritten('America/New_York', '2026-03-08', true)))).toBe(
			'2026-03-08'
		);
	});

	test('is kept as the moment it names when it carries one', () => {
		expect(instantWritten('Asia/Seoul', '2026-08-31T09:30:00+09:00')).toBe('2026-08-31T00:30:00.000Z');
	});

	test('is refused when it is not a date at all', () => {
		expect(() => instantWritten('Asia/Seoul', 'next tuesday')).toThrow('not a date');
	});
});

describe('a label a company registered', () => {
	const labels = labelsOfVocabulary(
		{ businesses: [{ name: '영업' }, { name: '개발' }], types: [{ name: '문서' }] },
		'Asia/Seoul'
	);

	test('is read from the company’s own vocabulary', () => {
		expect(labels.businesses).toEqual(['영업', '개발']);
		expect(labels.types).toEqual(['문서']);
	});

	test('falls back to the company timezone it was given', () => {
		expect(labelsOfVocabulary({}, null).timezone).toBe('Asia/Seoul');
		expect(labelsOfVocabulary({}, 'UTC').timezone).toBe('UTC');
	});

	test('is matched whole, or by a part only one label answers to', () => {
		expect(labelOf(labels.businesses, '영업', null)).toBe('영업');
		expect(labelOf(labels.businesses, '개', null)).toBe('개발');
	});

	test('is refused when nothing registered answers to it', () => {
		expect(() => labelOf(labels.businesses, '마케팅', null)).toThrow(LabelUnresolved);
	});

	test('is left alone when the caller said nothing, and emptied when they said nothing at all', () => {
		expect(labelOf(labels.businesses, undefined, '영업')).toBe('영업');
		expect(labelOf(labels.businesses, '  ', '영업')).toBeNull();
	});
});
