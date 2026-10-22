import { messengerCacheScope, onMessengerCacheReset, requireCurrentMessengerScope } from './cache-scope';
import { fetchPeople, type MessengerPeopleList } from './messenger-api';

let loading: { key: string; generation: number; value: Promise<MessengerPeopleList> } | null = null;

export async function fetchMessengerAccounts(): Promise<MessengerPeopleList> {
	const scope = await messengerCacheScope();
	if (loading?.key === scope.key && loading.generation === scope.generation) return loading.value;
	const value = fetchPeople().then(async (accounts) => {
		await requireCurrentMessengerScope(scope);
		return accounts;
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
