import { describe, expect, test } from 'bun:test';
import { calendarAuditRows } from '../../../src/routes/calendar/embed/calendar-audit';

describe('calendar audit rows', () => {
	test('uses provided labels and locale for audit details', () => {
		const rows = calendarAuditRows(
			{
				meta: {
					createdByName: 'Creator',
					updatedByName: 'Updater',
					updatedByAt: '2026-06-17T01:30:00.000Z'
				}
			},
			'en-US',
			{ created: 'Created', updated: 'Updated' }
		);

		expect(rows[0]).toEqual({ label: 'Created', person: 'Creator', time: '' });
		expect(rows[1]?.label).toBe('Updated');
		expect(rows[1]?.person).toBe('Updater');
		expect(rows[1]?.time.includes('2026')).toBe(true);
	});
});
