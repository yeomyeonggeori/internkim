import { supabase } from '$lib/supabase';
import { supabaseMember } from '$lib/supabase-session';
import type { CalendarSyncResponse } from '../../routes/calendar/calendar-layout-types';

const feedCredentialKind = 'calendar_feed';
const octetsInAFeedKey = 32;

export async function supabaseCalendarSubscription(): Promise<CalendarSyncResponse | null> {
	const { memberID } = await supabaseMember();
	if (!memberID) return null;
	const held = await heldFeedKey(memberID);
	return held ? { icsURL: feedURLOf(held) } : null;
}

export async function issueSupabaseCalendarSubscription(): Promise<CalendarSyncResponse | null> {
	const { memberID } = await supabaseMember();
	if (!memberID) return null;
	const held = await heldFeedKey(memberID);
	return { icsURL: feedURLOf(held || (await writeFeedKey(memberID))) };
}

export async function rotateSupabaseCalendarSubscription(): Promise<CalendarSyncResponse> {
	const { memberID } = await supabaseMember();
	if (!memberID) throw new Error('this page is not signed in as a member of a company');
	return { icsURL: feedURLOf(await writeFeedKey(memberID)) };
}

async function heldFeedKey(memberID: string): Promise<string> {
	const held = await supabase()
		.from('credential')
		.select('external_id')
		.eq('member_id', memberID)
		.eq('kind', feedCredentialKind)
		.maybeSingle<{ external_id: string | null }>();
	if (held.error) throw new Error(held.error.message);
	return held.data?.external_id ?? '';
}

async function writeFeedKey(memberID: string): Promise<string> {
	const key = newFeedKey();
	const written = await supabase().from('credential').upsert(
		{
			member_id: memberID,
			kind: feedCredentialKind,
			name: '',
			external_id: key,
			permission: 'read'
		},
		{ onConflict: 'member_id,kind,name' }
	);
	if (written.error) throw new Error(written.error.message);
	return key;
}

function feedURLOf(key: string): string {
	return `${window.location.origin}/calendar/feed/${key}.ics`;
}

function newFeedKey(): string {
	return [...crypto.getRandomValues(new Uint8Array(octetsInAFeedKey))]
		.map((octet) => octet.toString(16).padStart(2, '0'))
		.join('');
}
