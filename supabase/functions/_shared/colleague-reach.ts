import type { AttendanceReach } from './categories.ts';

export type Placed = {
	id: string;
	teamID: string | null;
	supervisorID: string | null;
};

export function isWithinReach(
	reach: AttendanceReach,
	listener: Placed,
	announcer: Placed,
	placedByID: Map<string, Placed>
): boolean {
	if (reach === 'everyone') return true;
	if (sharesTeam(listener, announcer)) return true;
	if (reach === 'team') return false;
	return supervisorsAbove(listener, placedByID).has(announcer.id);
}

function sharesTeam(listener: Placed, announcer: Placed): boolean {
	return listener.teamID !== null && listener.teamID === announcer.teamID;
}

function supervisorsAbove(person: Placed, placedByID: Map<string, Placed>): Set<string> {
	const above = new Set<string>();
	let supervisor = person.supervisorID ? placedByID.get(person.supervisorID) : undefined;
	while (supervisor && supervisor.id !== person.id && !above.has(supervisor.id)) {
		above.add(supervisor.id);
		supervisor = supervisor.supervisorID ? placedByID.get(supervisor.supervisorID) : undefined;
	}
	return above;
}
