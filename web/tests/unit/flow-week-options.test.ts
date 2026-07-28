import { describe, expect, test } from 'bun:test';

import { flowWeekOptions } from '../../src/routes/flow/flow-week-label';

describe('flow week options', () => {
	test('lists weeks around the current one, newest week last', () => {
		const options = flowWeekOptions('2026-06-01', 2, 1);

		expect(options.map((option) => option.label)).toEqual(['5/18 - 5/24', '5/25 - 5/31', '6/1 - 6/7', '6/8 - 6/14']);
		expect(options.filter((option) => option.isCurrent).map((option) => option.label)).toEqual(['6/1 - 6/7']);
	});

	test('returns nothing without a valid week start', () => {
		expect(flowWeekOptions('', 2, 1)).toEqual([]);
		expect(flowWeekOptions('not-a-date', 2, 1)).toEqual([]);
	});
});
