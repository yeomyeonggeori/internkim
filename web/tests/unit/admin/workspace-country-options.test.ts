import { expect, test } from 'bun:test';

import { workspaceCountryOptions } from '../../../src/routes/admin/workspace-country-options';

test('workspace country options use ISO alpha-2 values and localized labels', () => {
	const options = workspaceCountryOptions(
		[
			{ countryCode: 'KR', name: 'South Korea' },
			{ countryCode: 'US', name: 'United States' }
		],
		'ko'
	);
	const korea = options.find((option) => option.value === 'KR');
	const unitedStates = options.find((option) => option.value === 'US');

	expect(korea?.label.endsWith('(KR)')).toBe(true);
	expect(unitedStates?.label.endsWith('(US)')).toBe(true);
	expect(options.every((option) => /^[A-Z]{2}$/.test(option.value))).toBe(true);
});

test('workspace country options only include countries supported by the holiday API', () => {
	const options = workspaceCountryOptions([{ countryCode: 'KR', name: 'South Korea' }], 'en');

	expect(options.map((option) => option.value)).toEqual(['KR']);
});
