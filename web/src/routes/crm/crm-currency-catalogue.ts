import { isSupabaseConfigured } from '$lib/supabase';
import { crmInterimCurrencyCatalogue, type CRMCurrencyCatalogue } from './crm-money';

export async function loadCRMCurrencyCatalogue(): Promise<CRMCurrencyCatalogue> {
	if (!isSupabaseConfigured()) return crmInterimCurrencyCatalogue;
	const response = await fetch('/api/crm/currencies');
	if (!response.ok) return crmInterimCurrencyCatalogue;
	const payload = (await response.json()) as { currencies?: CRMCurrencyCatalogue };
	const currencies = payload.currencies;
	return currencies && currencies.length > 0 ? currencies : crmInterimCurrencyCatalogue;
}
