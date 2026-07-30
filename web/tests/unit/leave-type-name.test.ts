import { describe, expect, test } from 'bun:test';
import { localizedLeaveTypeName } from '../../src/lib/i18n/leave-type-name';

describe('localized leave type name', () => {
	test('localizes canonical system leave names', () => {
		expect(localizedLeaveTypeName('annual', '연차', 'en')).toBe('Annual leave');
		expect(localizedLeaveTypeName('annual', 'Annual leave', 'ko')).toBe('연차');
		expect(localizedLeaveTypeName('legacy-leave', 'Legacy leave', 'ko')).toBe('기존 휴가');
		expect(localizedLeaveTypeName('legacy-leave', '기존 휴가', 'en')).toBe('Legacy leave');
		expect(localizedLeaveTypeName('quarter-day', '반반차', 'en')).toBe('반반차');
	});

	test('preserves administrator and custom leave names', () => {
		expect(localizedLeaveTypeName('annual', '유급 연차', 'en')).toBe('유급 연차');
		expect(localizedLeaveTypeName('custom-family', 'Family day', 'ko')).toBe('Family day');
	});
});
