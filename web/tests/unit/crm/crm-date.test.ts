import { describe, expect, test } from 'bun:test';
import { buildCRMReportPeriodBounds } from '../../../src/routes/crm/crm-date';

describe('CRM report period dates', () => {
	test('uses the selected time zone for local day and quarter boundaries', () => {
		const now = new Date('2026-04-01T00:30:00Z');

		expect(buildCRMReportPeriodBounds(now, 'Asia/Seoul')).toEqual({
			today: '2026-04-01',
			next90DaysEnd: '2026-06-30',
			quarterStart: '2026-04-01',
			quarterEnd: '2026-06-30'
		});
		expect(buildCRMReportPeriodBounds(now, 'America/New_York')).toEqual({
			today: '2026-03-31',
			next90DaysEnd: '2026-06-29',
			quarterStart: '2026-01-01',
			quarterEnd: '2026-03-31'
		});
	});
});
