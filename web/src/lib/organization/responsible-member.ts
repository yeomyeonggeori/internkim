import type { UserRecord } from './types';

export function responsibleMemberIDOf(groupRecords: UserRecord[]): string | undefined {
	const memberIDs = new Set(groupRecords.map((record) => record.memberID));
	const candidates = groupRecords.filter((record) => {
		const supervisorID = record.supervisorID?.trim() ?? '';
		return !supervisorID || !memberIDs.has(supervisorID);
	});
	return candidates.length === 1 ? candidates[0].memberID : undefined;
}
