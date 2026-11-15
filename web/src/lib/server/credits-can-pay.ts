import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import type { ModelKeyReport } from '$lib/company/credits-contract';

const modelKeyReportRows = z.array(z.object({ holds_model_key: z.boolean().nullable(), uses_own_model_key_at_start: z.boolean().nullable() }));

export async function creditsCanPayForModels(service: SupabaseClient, companyID: string): Promise<boolean> {
	const { data, error } = await service.rpc('company_credits_can_pay', { target_company: companyID });
	if (error) throw new Error(`whether credits can pay for ${companyID}'s models: ${error.message}`);
	return z.boolean().parse(data);
}

export async function modelKeyReportOf(service: SupabaseClient, companyID: string): Promise<ModelKeyReport> {
	const { data, error } = await service.rpc('company_model_key_report', { target_company: companyID });
	if (error) throw new Error(`what ${companyID}'s computer said about its model key: ${error.message}`);
	const [reported] = modelKeyReportRows.parse(data ?? []);
	return reported ? { holdsModelKey: reported.holds_model_key, usesOwnModelKeyAtStart: reported.uses_own_model_key_at_start } : null;
}
