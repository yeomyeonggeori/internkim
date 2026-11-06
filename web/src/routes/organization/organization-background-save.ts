import type { UserRecord } from '../../lib/organization/types';

export function replaceOrganizationRecord(records: UserRecord[], replacement: UserRecord): UserRecord[] {
	return records.map((record) => (record.memberID === replacement.memberID ? replacement : record));
}
