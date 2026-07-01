import type { UserRecord } from './admin-types';

export type OrgNode = {
	record: UserRecord;
	reports: OrgNode[];
};

export function orgForest(userRecords: UserRecord[]): OrgNode[] {
	const reportsBySupervisor = new Map<string, UserRecord[]>();
	const knownID = new Set(userRecords.map((record) => record.userID));
	for (const record of userRecords) {
		const supervisorID = record.supervisorID?.trim() ?? '';
		const key = supervisorID && knownID.has(supervisorID) ? supervisorID : '';
		if (!reportsBySupervisor.has(key)) reportsBySupervisor.set(key, []);
		reportsBySupervisor.get(key)?.push(record);
	}
	const placed = new Set<string>();
	const buildNode = (record: UserRecord): OrgNode => {
		placed.add(record.userID);
		const reports = (reportsBySupervisor.get(record.userID) ?? [])
			.filter((report) => !placed.has(report.userID))
			.sort(byHireDate)
			.map(buildNode);
		return { record, reports };
	};
	const roots = (reportsBySupervisor.get('') ?? []).sort(byHireDate).map(buildNode);
	for (const record of userRecords.filter((candidate) => !placed.has(candidate.userID)).sort(byHireDate)) {
		if (!placed.has(record.userID)) roots.push(buildNode(record));
	}
	return roots;
}

export function supervisorCandidatesForRecord(userRecords: UserRecord[], record: UserRecord): UserRecord[] {
	const blockedUserIDs = new Set([record.userID, ...descendantUserIDs(userRecords, record.userID)]);
	return userRecords.filter((candidate) => !blockedUserIDs.has(candidate.userID)).sort(byHireDate);
}

export function isSupervisorCandidateForRecord(userRecords: UserRecord[], record: UserRecord): boolean {
	const supervisorID = record.supervisorID?.trim() ?? '';
	if (!supervisorID) return true;
	return supervisorCandidatesForRecord(userRecords, record).some((candidate) => candidate.userID === supervisorID);
}

function descendantUserIDs(userRecords: UserRecord[], userID: string): string[] {
	const reportsBySupervisor = new Map<string, UserRecord[]>();
	for (const record of userRecords) {
		const supervisorID = record.supervisorID?.trim() ?? '';
		if (!supervisorID) continue;
		if (!reportsBySupervisor.has(supervisorID)) reportsBySupervisor.set(supervisorID, []);
		reportsBySupervisor.get(supervisorID)?.push(record);
	}
	const descendants: string[] = [];
	const visit = (supervisorID: string) => {
		for (const report of reportsBySupervisor.get(supervisorID) ?? []) {
			if (descendants.includes(report.userID)) continue;
			descendants.push(report.userID);
			visit(report.userID);
		}
	};
	visit(userID);
	return descendants;
}

function byHireDate(first: UserRecord, second: UserRecord): number {
	const hireDateDifference = hireDateSortValue(first).localeCompare(hireDateSortValue(second));
	if (hireDateDifference !== 0) return hireDateDifference;
	return personLabel(first).localeCompare(personLabel(second));
}

function hireDateSortValue(record: UserRecord): string {
	return record.hireDate || '9999-12-31';
}

function personLabel(record: UserRecord): string {
	return record.name || record.email;
}
