export type DeviceAttendance = {
	kind: string;
	occurredAt: string;
	location: string;
};

export type RecordedAttendance = {
	id: string;
	kind: string;
	occurred_at: string;
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
	for (const event of fromDevice) wanted.set(keyOf(event.kind, event.occurredAt), event);

	const held = new Set(inRecord.map((row) => keyOf(row.kind, row.occurred_at)));

	const add = [...wanted]
		.filter(([key]) => !held.has(key))
		.map(([, event]) => ({
			member_id: memberID,
			kind: event.kind,
			occurred_at: event.occurredAt,
			location: locationFor(event)
		}));
	const remove = inRecord.filter((row) => !wanted.has(keyOf(row.kind, row.occurred_at))).map((row) => row.id);

	return { add, remove };
}

function keyOf(kind: string, moment: string): string {
	return `${kind}|${new Date(moment).toISOString().slice(0, 19)}`;
}

function locationFor(event: DeviceAttendance): string | null {
	if (event.kind !== 'clock_in') return null;
	return event.location.trim() === '' ? null : event.location;
}
