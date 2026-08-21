import { isSupabaseConfigured, supabase } from '$lib/supabase';

export const interimCompanyBaseCurrency = 'KRW';

type CompanyCurrencyRow = { id: string; currency_code: string };

async function readCompanyCurrency(): Promise<CompanyCurrencyRow> {
	const company = await supabase()
		.from('company')
		.select('id, currency_code')
		.limit(1)
		.single<CompanyCurrencyRow>();
	if (company.error) throw new Error(company.error.message);
	return company.data;
}

export async function loadCompanyBaseCurrency(): Promise<string> {
	if (!isSupabaseConfigured()) return interimCompanyBaseCurrency;
	return (await readCompanyCurrency()).currency_code;
}

export async function saveCompanyBaseCurrency(currency: string): Promise<void> {
	const company = await readCompanyCurrency();
	const saved = await supabase().from('company').update({ currency_code: currency }).eq('id', company.id).select('id');
	if (saved.error) throw new Error(saved.error.message);
	if ((saved.data ?? []).length === 0) throw new Error('only an administrator can change the base currency');
}
