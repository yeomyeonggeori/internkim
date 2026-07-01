import type { AdminPageText, CircleRecord, UserRecord, UserRole } from './admin-types';

export type UserRoleOption = {
	value: UserRole;
	label: string;
};

export function userAdminCount(records: UserRecord[]): number {
	return records.filter((record) => record.role === 'admin').length;
}

export function allUserRoleOptions(text: AdminPageText): UserRoleOption[] {
	return [
		{ value: 'member', label: text.users.member },
		{ value: 'operationsAdmin', label: text.users.operationsAdmin },
		{ value: 'admin', label: text.users.admin }
	];
}

export function userRoleOptions(text: AdminPageText, canGrantAdminRole: boolean): UserRoleOption[] {
	return allUserRoleOptions(text).filter((option) => canGrantAdminRole || option.value !== 'admin');
}

export function userRoleLabel(text: AdminPageText, role: UserRole): string {
	return allUserRoleOptions(text).find((option) => option.value === role)?.label ?? role;
}

export function canManageUserRecord(canGrantAdminRole: boolean, record: UserRecord): boolean {
	return canGrantAdminRole || record.role !== 'admin';
}

export function visibleUserCircles(circles: CircleRecord[]): CircleRecord[] {
	return circles.filter((circle) => circle.circleID !== 'admin');
}

export function isReservedCircleID(circleID: string): boolean {
	return ['staff', 'admin'].includes(circleID.trim().toLowerCase());
}

export function normalizeHandle(handle: string): string {
	return handle.trim().toLowerCase();
}

export function isValidHandle(handle: string): boolean {
	return /^[a-z][a-z0-9._-]{2,21}$/.test(normalizeHandle(handle));
}

export function isValidUserRecord(record: UserRecord): boolean {
	return isValidHandle(record.handle) && !!record.name?.trim() && !!record.email.trim();
}

export function sortUserRecordsByHireDate(records: UserRecord[]): UserRecord[] {
	return [...records].sort((first, second) => {
		if (first.hireDate || second.hireDate) {
			if (!first.hireDate) return 1;
			if (!second.hireDate) return -1;
			if (first.hireDate !== second.hireDate) return first.hireDate.localeCompare(second.hireDate);
		}
		return (first.name || first.email).localeCompare(second.name || second.email);
	});
}

export function normalizeUserCircles(circles: string[] | undefined, role: UserRole): string[] {
	const result = new Set(['staff', ...(circles ?? []).map((circle) => circle.trim().toLowerCase()).filter(Boolean)]);
	if (role === 'admin') result.add('admin');
	return [...result];
}

export function hasUserCircle(record: UserRecord, circleID: string): boolean {
	return normalizeUserCircles(record.circles, record.role).includes(circleID);
}

export function nextUserCircles(record: UserRecord, circleID: string): string[] {
	const current = new Set(normalizeUserCircles(record.circles, record.role));
	if (current.has(circleID)) current.delete(circleID);
	else current.add(circleID);
	return normalizeUserCircles([...current], record.role);
}
