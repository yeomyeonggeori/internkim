import { describe, expect, test } from 'bun:test';
import { personName } from '$lib/person-name';
import type { Locale } from '$lib/i18n/locale.svelte';
import sharedCases from '$lib/person-name-cases.json';

describe('personName', () => {
	test('reads every recorded name as the shared cases say', () => {
		expect(sharedCases.cases.length > 0).toBe(true);
		for (const sharedCase of sharedCases.cases) {
			expect(personName(sharedCase.recorded, sharedCase.locale as Locale)).toBe(sharedCase.rendered);
		}
	});
});
