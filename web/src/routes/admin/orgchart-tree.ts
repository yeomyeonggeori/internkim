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
			.sort(byPositionLevel)
			.map(buildNode);
		return { record, reports };
	};
	const roots = (reportsBySupervisor.get('') ?? []).sort(byPositionLevel).map(buildNode);
	for (const record of userRecords.filter((candidate) => !placed.has(candidate.userID)).sort(byPositionLevel)) {
		if (!placed.has(record.userID)) roots.push(buildNode(record));
	}
	return roots;
}

function byPositionLevel(first: UserRecord, second: UserRecord): number {
	const levelDifference = positionLevelSortValue(first) - positionLevelSortValue(second);
	if (levelDifference !== 0) return levelDifference;
	return personLabel(first).localeCompare(personLabel(second));
}

function positionLevelSortValue(record: UserRecord): number {
	return record.positionLevel ?? Number.MAX_SAFE_INTEGER;
}

function personLabel(record: UserRecord): string {
	return record.name || record.email;
}
