import { describe, expect, test } from 'bun:test';
import { dataRoomCompanyText } from '../../../src/lib/data-room/company-text';

describe('company page metric names', () => {
	test('Korean names the monthly revenue and active client metrics', () => {
		expect(dataRoomCompanyText.ko.metricNames.monthlyRevenue).toBe('월 매출');
		expect(dataRoomCompanyText.ko.metricNames.activeClients).toBe('활성 고객사');
	});

	test('every metric named in one language is named in the other', () => {
		expect(Object.keys(dataRoomCompanyText.ko.metricNames).sort()).toEqual(Object.keys(dataRoomCompanyText.en.metricNames).sort());
	});
});
