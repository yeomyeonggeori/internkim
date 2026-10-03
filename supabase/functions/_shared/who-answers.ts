import type { SupabaseClient } from './service-client.ts';

export async function whoAnswersFor(
	record: SupabaseClient,
	companyID: string,
	askerID: string
): Promise<string[]> {
	const administrators = await administratorsOf(record, companyID);
	return administrators.filter((memberID) => memberID !== askerID);
}

async function administratorsOf(record: SupabaseClient, companyID: string): Promise<string[]> {
	const { data, error } = await record
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.eq('is_admin', true)
		.neq('status', 'withdrawn')
		.returns<{ id: string }[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((member) => member.id);
}
