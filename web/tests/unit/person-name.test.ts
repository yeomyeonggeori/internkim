import { describe, expect, test } from 'bun:test';
import { personName, personNameLocale, redactedPersonName } from '$lib/person-name';
import type { Locale } from '$lib/i18n/locale.svelte';
import sharedCases from '$lib/person-name-cases.json';

describe('personName', () => {
	test('uses Korean name order when either company or UI language is Korean', () => {
		for (const companyLocale of ['ko', 'ko-KR', ' KO ']) {
			expect(personName('샘플 이', personNameLocale('en', companyLocale))).toBe('이샘플');
		}
		expect(personName('샘플 이', personNameLocale('ko', 'en'))).toBe('이샘플');
		expect(personName('샘플 이', personNameLocale('en', 'en'))).toBe('샘플 이');
		expect(personName('Sample Person', personNameLocale('ko', 'ko'))).toBe('Sample Person');
	});
	test('reads every recorded name as the shared cases say', () => {
		expect(sharedCases.cases.length > 0).toBe(true);
		for (const sharedCase of sharedCases.cases) {
			expect(personName(sharedCase.recorded, sharedCase.locale as Locale)).toBe(sharedCase.rendered);
		}
	});
});

describe('redactedPersonName', () => {
	test('puts a kept Korean family name first for a Korean reader and leaves the record order otherwise', () => {
		expect(redactedPersonName('•• 곽', 'ko')).toBe('곽••');
		expect(redactedPersonName('•• 곽', 'en')).toBe('•• 곽');
		expect(redactedPersonName('•••••• Kim', 'ko')).toBe('•••••• Kim');
		expect(redactedPersonName('이••', 'ko')).toBe('이••');
	});
});
