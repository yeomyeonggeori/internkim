import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
import type { OrgGroup, UserRecord } from '$lib/organization/types';
import { isSupabaseConfigured } from '$lib/supabase';
import type { MentionPerson } from './mention-candidates';
import { fetchMessengerDirectory } from './messenger-directory';

export type PersonProfileRecord = {
	record: UserRecord;
	groups: OrgGroup[];
	userRecords: UserRecord[];
};

export async function personProfileRecordOf(person: MentionPerson): Promise<PersonProfileRecord | undefined> {
	if (!isSupabaseConfigured()) return undefined;
	const memberID = person.memberID || (await fetchMessengerDirectory()).memberOfExternal.get(person.externalID);
	if (!memberID) return undefined;
	const directory = await supabaseOrganizationDirectory();
	const userRecords = directory.records ?? [];
	const record = userRecords.find((candidate) => candidate.memberID === memberID);
	if (!record) return undefined;
	return { record, groups: directory.availableGroups ?? [], userRecords };
}
