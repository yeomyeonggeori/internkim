import { isSupabaseConfigured } from '$lib/supabase';
import { saveCompanySettings } from './company-settings';
import { invokeTool } from '$lib/public-api-call';

export const interimCompanyBaseCurrency = 'KRW';

export async function loadCompanyBaseCurrency(): Promise<string> {
	if (!isSupabaseConfigured()) return interimCompanyBaseCurrency;
	return (await invokeTool<{ currencyCode: string }>('company_settings_get', { includeProfileImage: false })).currencyCode;
}

export async function saveCompanyBaseCurrency(currency: string): Promise<void> {
	await saveCompanySettings({ currencyCode: currency });
}
