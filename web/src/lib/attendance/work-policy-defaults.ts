import type { CurrentAttendanceWorkPolicy } from './current-work-policy';

export const initialWorkPolicyEffectiveDate = '1970-01-01';

export function defaultWorkPolicy(): CurrentAttendanceWorkPolicy {
	return {
		workMode: 'flexible',
		workingWeekdays: [1, 2, 3, 4, 5],
		dailyTargetMinutes: 8 * 60,
		weeklyTargetMinutes: 40 * 60,
		referenceStartTime: '09:00',
		fixedStartTime: '',
		fixedEndTime: '',
		coreTimeEnabled: true,
		coreStartTime: '11:00',
		coreEndTime: '16:00',
		breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
		nightStartTime: '22:00',
		nightEndTime: '06:00'
	};
}
