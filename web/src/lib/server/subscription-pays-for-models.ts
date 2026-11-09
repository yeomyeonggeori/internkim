import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';

const payingStatuses = new Set(['active', 'trialing']);

const subscriptionRow = z.object({ subscription_status: z.string().nullable() });

export async function subscriptionPaysForModels(client: SupabaseClient, companyID: string): Promise<boolean> {
	const { data, error } = await client.from('company').select('subscription_status').eq('id', companyID).single();
	if (error) throw new Error(`whether a subscription pays for ${companyID}'s models: ${error.message}`);
	return payingStatuses.has(subscriptionRow.parse(data).subscription_status ?? '');
}
