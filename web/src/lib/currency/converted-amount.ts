import { isSupabaseConfigured } from '$lib/supabase';
import type { Money } from '$lib/money';

export type ConvertedAmount = { amount: Money; rate: number; asOf: string };

export async function loadConvertedAmount(money: Money, to: string): Promise<ConvertedAmount | null> {
	if (!isSupabaseConfigured()) return null;
	if (!to || money.currency === to) return null;
	if (!Number.isFinite(money.amount) || money.amount <= 0) return null;
	const searchParameters = new URLSearchParams({ amount: String(money.amount), from: money.currency, to });
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
