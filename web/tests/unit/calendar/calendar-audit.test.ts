import { describe, expect, test } from 'bun:test';
import { calendarAuditRows } from '../../../src/routes/calendar/embed/calendar-audit';

describe('calendar audit rows', () => {
	test('uses provided labels and locale for audit details', () => {
		const rows = calendarAuditRows(
			{
				meta: {
					createdByName: 'Creator',
					createdByEmail: 'creator@example.com',
					createdByImage: '/calendar/api/events/event-1/actor-image?actor=created',
					updatedByName: 'Updater',
					updatedByEmail: 'updater@example.com',
					updatedByAt: '2026-06-17T01:30:00.000Z'
				}
			},
			'en-US',
			{ created: 'Created', updated: 'Updated' }
		);

		expect(rows[0]).toEqual({
			label: 'Created',
			actor: {
				name: 'Creator',
				email: 'creator@example.com',
				image: '/calendar/api/events/event-1/actor-image?actor=created'
			},
			time: ''
		});
		expect(rows[1]?.label).toBe('Updated');
		expect(rows[1]?.actor.name).toBe('Updater');
		expect(rows[1]?.actor.email).toBe('updater@example.com');
		expect(rows[1]?.time.includes('2026')).toBe(true);
	});
});
