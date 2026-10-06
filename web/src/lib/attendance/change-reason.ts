export const attendanceChangeReasons = ['record_missing', 'time_correction', 'location_correction', 'other'] as const;
export type AttendanceChangeReason = typeof attendanceChangeReasons[number];
export function isAttendanceChangeReason(value: string): value is AttendanceChangeReason { return (attendanceChangeReasons as readonly string[]).includes(value); }
export const attendanceChangeReasonLabels = {
 ko: {record_missing: '기록 누락', time_correction: '시간 정정', location_correction: '근무지 정정', other: '기타'},
 en: {record_missing: 'Missing record', time_correction: 'Time correction', location_correction: 'Work location correction', other: 'Other'}
};
// Existing reason text remains readable. New API callers use the explicit stable reasonCode field.
export function attendanceChangeReasonText(code: AttendanceChangeReason): string { return attendanceChangeReasonLabels.ko[code]; }

export function attendanceChangeReasonOf(value: string | null | undefined): AttendanceChangeReason {
 if (value && isAttendanceChangeReason(value)) return value;
 if (value === '기록 누락' || value === '누락된 기록 추가') return 'record_missing';
 if (value === '시간 정정' || value === '시각 정정') return 'time_correction';
 if (value === '근무지 정정') return 'location_correction';
 return 'other';
}
export function attendanceChangeReasonLabel(value: string | null | undefined, locale: string): string {
 return attendanceChangeReasonLabels[locale === 'ko' ? 'ko' : 'en'][attendanceChangeReasonOf(value)];
}
