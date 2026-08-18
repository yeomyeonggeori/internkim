export type DeviceAttendance = {
	kind: string;
	occurredAt: string;
	location: string;
};

export type RecordedAttendance = {
	id: string;
	kind: string;
	occurred_at: string;
	original_occurred_at: string | null;
};

export type Reconciliation = {
	add: { member_id: string; kind: string; occurred_at: string; location: string | null }[];
	remove: string[];
};

export class EmptyWindowRefused extends Error {
	constructor(memberID: string, held: number) {
		super(`the device reports nothing for ${memberID} in a window the record holds ${held} of`);
		this.name = 'EmptyWindowRefused';
	}
}

export function reconcileMember(
	memberID: string,
	fromDevice: DeviceAttendance[],
	inRecord: RecordedAttendance[]
): Reconciliation {
	if (fromDevice.length === 0 && inRecord.length > 0) throw new EmptyWindowRefused(memberID, inRecord.length);

	const wanted = new Map<string, DeviceAttendance>();
	for (const event of paired(fromDevice)) wanted.set(keyOf(event.kind, event.occurredAt), event);

	const held = new Set(inRecord.map((row) => keyOf(row.kind, identityMomentOf(row))));

	const add = [...wanted]
		.filter(([key]) => !held.has(key))
		.map(([, event]) => ({
			member_id: memberID,
			kind: event.kind,
			occurred_at: event.occurredAt,
			location: locationFor(event)
		}));
	const remove = inRecord.filter((row) => !wanted.has(keyOf(row.kind, identityMomentOf(row)))).map((row) => row.id);

	return { add, remove };
}

export function attendanceIdentityWindowFilter(from: string, to: string): string {
	return [
		`and(original_occurred_at.gte.${from},original_occurred_at.lt.${to})`,
		`and(original_occurred_at.is.null,occurred_at.gte.${from},occurred_at.lt.${to})`
	].join(',');
}

function identityMomentOf(row: RecordedAttendance): string {
	return row.original_occurred_at ?? row.occurred_at;
}

function keyOf(kind: string, moment: string): string {
	return `${kind}|${new Date(moment).toISOString().slice(0, 19)}`;
}

function locationFor(event: DeviceAttendance): string | null {
	if (event.kind !== 'clock_in') return null;
	return event.location.trim() === '' ? null : event.location;
}

export function paired(fromDevice: DeviceAttendance[]): DeviceAttendance[] {
	const inOrder = [...fromDevice].sort((first, second) => first.occurredAt.localeCompare(second.occurredAt));
	return inOrder.filter((event, index) => {
		if (event.kind !== 'clock_in') return true;
		const next = inOrder[index + 1];
		if (!next || next.kind !== 'clock_in') return true;
		return next.location !== event.location;
	});
}
