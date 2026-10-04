import type {
	RecordAttendanceList,
	RecordCompanySettings,
	RecordDirectory,
	RecordLeaveList
} from './attendance-record';

// A snapshot belongs to one live summary. Symbol keys are not persisted with
// the display cache, so a restored summary cannot skip permission revalidation.
export const attendanceSummaryRecords = Symbol('attendance-summary-records');

export type AttendanceSummaryRecords = {
	readScope?: 'all' | 'mine';
	settings: RecordCompanySettings;
	directory: RecordDirectory;
	attendance: RecordAttendanceList;
	leave: RecordLeaveList;
	from: string;
	to: string;
};
