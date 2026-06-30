import type { UserRecord, UserRole } from './admin-types';

export type UserRecordChanges = Partial<Pick<UserRecord, 'handle' | 'name' | 'hireDate' | 'note'>>;

export function normalizeUserCircles(circles: string[] | undefined, role: UserRole): string[] {
	const result = new Set(['staff', ...(circles ?? []).map((circle) => circle.trim().toLowerCase()).filter(Boolean)]);
	if (role === 'admin') result.add('admin');
	return [...result];
}

export function hasUserCircle(record: UserRecord, circleID: string): boolean {
	return normalizeUserCircles(record.circles, record.role).includes(circleID);
}

export function updateUserRecords(records: UserRecord[], email: string, changes: UserRecordChanges): UserRecord[] {
	return records.map((record) => record.email === email ? { ...record, ...changes } : record);
}

export function toggleUserRecordCircle(records: UserRecord[], email: string, circleID: string): UserRecord[] {
	if (circleID === 'staff') return records;
	return records.map((record) => {
		if (record.email !== email) return record;
		const current = new Set(normalizeUserCircles(record.circles, record.role));
		if (current.has(circleID)) current.delete(circleID);
		else current.add(circleID);
		return { ...record, circles: normalizeUserCircles([...current], record.role) };
	});
}
