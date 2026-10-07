import { isSupabaseConfigured } from '$lib/supabase';

export type ConvertedAmount = { amountMinor: number; currencyCode: string; rate: number; asOf: string };

export async function loadConvertedAmount(amountMinor: number, from: string, to: string): Promise<ConvertedAmount | null> {
	if (!isSupabaseConfigured()) return null;
	if (!from || !to || from === to) return null;
	if (!Number.isSafeInteger(amountMinor) || amountMinor <= 0) return null;
	const searchParameters = new URLSearchParams({ amountMinor: String(amountMinor), from, to });
	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), 8000);
	try {
		const response = await fetch(`/api/currencies/conversion?${searchParameters.toString()}`, {
			signal: controller.signal
		});
		if (!response.ok) return null;
		return await response.json();
	} catch {
		return null;
	} finally {
		clearTimeout(timeout);
	}
}
