import { isSupabaseConfigured } from '$lib/supabase';
import { companySettings, saveCompanySettings } from './company-settings';

export const interimCompanyBaseCurrency = 'KRW';

export async function loadCompanyBaseCurrency(): Promise<string> {
	if (!isSupabaseConfigured()) return interimCompanyBaseCurrency;
	return (await companySettings()).currencyCode;
}

export async function saveCompanyBaseCurrency(currency: string): Promise<void> {
	await saveCompanySettings({ currencyCode: currency });
}
