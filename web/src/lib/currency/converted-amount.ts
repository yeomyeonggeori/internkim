import { isSupabaseConfigured, supabase } from '$lib/supabase';

export type ConvertedAmount = { amountMinor: number; currencyCode: string; rate: number; asOf: string };

export async function loadConvertedAmount(amountMinor: number, from: string, to: string): Promise<ConvertedAmount | null> {
	if (!isSupabaseConfigured()) return null;
	if (!from || !to || from === to) return null;
	if (!Number.isSafeInteger(amountMinor) || amountMinor <= 0) return null;
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) return null;
	const searchParameters = new URLSearchParams({ amountMinor: String(amountMinor), from, to });
	const response = await fetch(`/api/currencies/conversion?${searchParameters.toString()}`, {
		headers: { Authorization: `Bearer ${accessToken}` }
	});
	if (!response.ok) return null;
	return (await response.json()) as ConvertedAmount;
}
