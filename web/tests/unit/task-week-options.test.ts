import { describe, expect, test } from 'bun:test';

import { taskWeekOptions } from '../../src/routes/task/task-week-label';
import { taskWeekDateISO, taskWeekMondayForCode } from '../../src/lib/task/task-week-code';

describe('flow week options', () => {
	test('lists weeks around the current one, newest week last', () => {
		const options = taskWeekOptions('2026-06-01', 2, 1);

		expect(options.map((option) => option.label)).toEqual(['5/18 - 5/24', '5/25 - 5/31', '6/1 - 6/7', '6/8 - 6/14']);
		expect(options.map((option) => option.offsetFromCurrent)).toEqual([-2, -1, 0, 1]);
	});

	test('returns nothing without a valid week start', () => {
		expect(taskWeekOptions('', 2, 1)).toEqual([]);
		expect(taskWeekOptions('not-a-date', 2, 1)).toEqual([]);
	});

	test('offers values every week reader resolves back to the same Monday', () => {
		const options = taskWeekOptions('2026-06-01', 2, 1);
		const mondays = options.map((option) => {
			const monday = taskWeekMondayForCode(option.value);
			return monday ? taskWeekDateISO(monday) : '';
		});

		expect(mondays).toEqual(['2026-05-18', '2026-05-25', '2026-06-01', '2026-06-08']);
	});
});
