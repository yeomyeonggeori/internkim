import type { SupabaseClient } from '@supabase/supabase-js';

const representativeCircle = 'representative';

export async function whoAnswersFor(
	record: SupabaseClient,
	companyID: string,
	askerID: string
): Promise<string[]> {
	const representatives = await circleMembersOf(record, companyID, representativeCircle);
	const asked = representatives.length > 0 ? representatives : await administratorsOf(record, companyID);
	return asked.filter((memberID) => memberID !== askerID);
}

async function circleMembersOf(
	record: SupabaseClient,
	companyID: string,
	name: string
): Promise<string[]> {
	const { data, error } = await record
		.from('circle')
		.select('name, circle_member(member_id)')
		.eq('company_id', companyID)
		.eq('name', name)
		.returns<{ name: string; circle_member: { member_id: string }[] | null }[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).flatMap((circle) => (circle.circle_member ?? []).map((held) => held.member_id));
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
