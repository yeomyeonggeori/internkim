export const attendanceWorkModes = ['autonomous', 'flexible', 'fixed'] as const;

export type AttendanceWorkMode = (typeof attendanceWorkModes)[number];

export function isAttendanceWorkMode(value: unknown): value is AttendanceWorkMode {
	return attendanceWorkModes.some((workMode) => workMode === value);
}
