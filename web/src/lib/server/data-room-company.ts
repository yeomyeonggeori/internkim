import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { dataRoomCompanySchema, type DataRoomCompany } from '$lib/data-room/company';

export async function companyShownTo(caller: SupabaseClient, companyID: string): Promise<DataRoomCompany> {
	return answeredCompany(await caller.rpc('data_room_company', { target_company: companyID }));
}

export async function companyShownToCircle(caller: SupabaseClient, companyID: string, circleID: string): Promise<DataRoomCompany> {
	return answeredCompany(await caller.rpc('data_room_circle_company', { target_company: companyID, target_circle: circleID }));
}

function answeredCompany({ data, error: refusal }: { data: unknown; error: { message: string } | null }): DataRoomCompany {
	if (refusal) error(502, 'the company could not be read');
	return dataRoomCompanySchema.parse(data);
}
