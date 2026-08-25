import { compareOrganizationPeople } from '$lib/organization/person-order';
import type { UserRecord } from '../../lib/organization/types';

export type OrgNode = {
	record: UserRecord;
	reports: OrgNode[];
};

export function orgForest(userRecords: UserRecord[]): OrgNode[] {
	const reportsBySupervisor = new Map<string, UserRecord[]>();
	const knownID = new Set(userRecords.map((record) => record.memberID));
	for (const record of userRecords) {
		const supervisorID = record.supervisorID?.trim() ?? '';
		const key = supervisorID && knownID.has(supervisorID) ? supervisorID : '';
		if (!reportsBySupervisor.has(key)) reportsBySupervisor.set(key, []);
		reportsBySupervisor.get(key)?.push(record);
	}
	const placed = new Set<string>();
	const buildNode = (record: UserRecord): OrgNode => {
		placed.add(record.memberID);
		const reports = (reportsBySupervisor.get(record.memberID) ?? [])
			.filter((report) => !placed.has(report.memberID))
			.sort(byHireDate)
			.map(buildNode);
		return { record, reports };
	};
	const roots = (reportsBySupervisor.get('') ?? []).sort(byHireDate).map(buildNode);
	for (const record of userRecords.filter((candidate) => !placed.has(candidate.memberID)).sort(byHireDate)) {
		if (!placed.has(record.memberID)) roots.push(buildNode(record));
	}
	return roots;
}

export function supervisorCandidatesForRecord(userRecords: UserRecord[], record: UserRecord): UserRecord[] {
	const blockedUserIDs = new Set([record.memberID, ...descendantUserIDs(userRecords, record.memberID)]);
	return userRecords.filter((candidate) => !blockedUserIDs.has(candidate.memberID)).sort(byHireDate);
}

export function isSupervisorCandidateForRecord(userRecords: UserRecord[], record: UserRecord): boolean {
	const supervisorID = record.supervisorID?.trim() ?? '';
	if (!supervisorID) return true;
	return supervisorCandidatesForRecord(userRecords, record).some((candidate) => candidate.memberID === supervisorID);
}

function descendantUserIDs(userRecords: UserRecord[], memberID: string): string[] {
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
			if (descendants.includes(report.memberID)) continue;
			descendants.push(report.memberID);
			visit(report.memberID);
		}
	};
	visit(memberID);
	return descendants;
}

function byHireDate(first: UserRecord, second: UserRecord): number {
	return compareOrganizationPeople(first, second);
}
