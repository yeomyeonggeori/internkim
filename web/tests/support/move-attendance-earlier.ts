import type { SupabaseClient } from '@supabase/supabase-js';

export async function moveAttendanceEarlier(
	record: SupabaseClient,
	memberID: string,
	minutes: number
): Promise<void> {
	const held = await record
		.from('attendance')
		.select('id, occurred_at')
		.eq('member_id', memberID)
		.order('occurred_at', { ascending: true })
		.returns<{ id: string; occurred_at: string }[]>();
	if (held.error) throw new Error(`Failed to read attendance: ${held.error.message}`);
	for (const row of held.data) {
		const moved = await record
			.from('attendance')
			.update({
				occurred_at: new Date(Date.parse(row.occurred_at) - minutes * 60_000).toISOString(),
				edit_reason: 'moved earlier by the test'
			})
			.eq('id', row.id);
		if (moved.error) throw new Error(`Failed to move attendance earlier: ${moved.error.message}`);
	}
}
