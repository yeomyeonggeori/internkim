import type { AttendanceReach } from './categories.ts';
import { isWithinReach, type Placed } from './colleague-reach.ts';

export type Colleague = Placed & {
	attendanceReach: AttendanceReach;
};

export function whoHearsTheClockOf(announcerID: string, colleagues: Colleague[]): string[] {
	const colleagueByID = new Map(colleagues.map((colleague) => [colleague.id, colleague]));
	const announcer = colleagueByID.get(announcerID);
	if (!announcer) return [];
	return colleagues
		.filter(
			(listener) =>
				listener.id !== announcerID &&
				isWithinReach(listener.attendanceReach, listener, announcer, colleagueByID)
		)
		.map((listener) => listener.id);
}
