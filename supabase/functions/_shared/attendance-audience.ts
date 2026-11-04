import type { AttendanceReach } from './categories.ts';

export type Colleague = {
	id: string;
	teamID: string | null;
	supervisorID: string | null;
	attendanceReach: AttendanceReach;
};

export function whoHearsTheClockOf(announcerID: string, colleagues: Colleague[]): string[] {
	const colleagueByID = new Map(colleagues.map((colleague) => [colleague.id, colleague]));
	const announcer = colleagueByID.get(announcerID);
	if (!announcer) return [];
	return colleagues
		.filter((listener) => listener.id !== announcerID && hears(listener, announcer, colleagueByID))
		.map((listener) => listener.id);
}

function hears(listener: Colleague, announcer: Colleague, colleagueByID: Map<string, Colleague>): boolean {
	if (listener.attendanceReach === 'everyone') return true;
	if (sharesTeam(listener, announcer)) return true;
	if (listener.attendanceReach === 'team') return false;
	return supervisorsAbove(listener, colleagueByID).has(announcer.id);
}

function sharesTeam(listener: Colleague, announcer: Colleague): boolean {
	return listener.teamID !== null && listener.teamID === announcer.teamID;
}

function supervisorsAbove(person: Colleague, colleagueByID: Map<string, Colleague>): Set<string> {
	const above = new Set<string>();
	let supervisor = person.supervisorID ? colleagueByID.get(person.supervisorID) : undefined;
	while (supervisor && supervisor.id !== person.id && !above.has(supervisor.id)) {
		above.add(supervisor.id);
		supervisor = supervisor.supervisorID ? colleagueByID.get(supervisor.supervisorID) : undefined;
	}
	return above;
}
