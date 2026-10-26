import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
import type { OrgGroup, UserRecord, UsersResponse } from '$lib/organization/types';
import { isSupabaseConfigured } from '$lib/supabase';
import { messengerCacheScope, onMessengerCacheReset, requireCurrentMessengerScope } from './cache-scope';
import type { MentionPerson } from './mention-candidates';
import { fetchMessengerDirectory } from './messenger-directory';

export type PersonProfileRecord = {
	record: UserRecord;
	groups: OrgGroup[];
	userRecords: UserRecord[];
	representativeID?: string;
};

let loading: { key: string; generation: number; value: Promise<UsersResponse> } | null = null;

async function organizationDirectory(): Promise<UsersResponse> {
	const scope = await messengerCacheScope();
	if (loading?.key === scope.key && loading.generation === scope.generation) return loading.value;
	const value = supabaseOrganizationDirectory().then(async (directory) => {
		await requireCurrentMessengerScope(scope);
		return directory;
	});
	const lookup = { ...scope, value };
	loading = lookup;
	try {
		return await value;
	} catch (refusal) {
		if (loading === lookup) loading = null;
		throw refusal;
	}
}

onMessengerCacheReset(() => {
	loading = null;
});

export async function personProfileRecordOf(person: MentionPerson): Promise<PersonProfileRecord | undefined> {
	if (!isSupabaseConfigured()) return undefined;
	const memberID = person.memberID || (await fetchMessengerDirectory()).memberOfExternal.get(person.externalID);
	if (!memberID) return undefined;
	const directory = await organizationDirectory();
	const userRecords = directory.records ?? [];
	const record = userRecords.find((candidate) => candidate.memberID === memberID);
	if (!record) return undefined;
	return { record, groups: directory.availableGroups ?? [], userRecords, representativeID: directory.representativeID };
}
