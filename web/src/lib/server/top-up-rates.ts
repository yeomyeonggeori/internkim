import { z } from 'zod';

export type TopUpRates = Record<string, number>;

const topUpRates = z.record(z.string(), z.number().positive());

export function topUpRatesOf(setting: string | undefined): TopUpRates | null {
	if (!setting) return null;
	const rates = topUpRates.safeParse(jsonOf(setting));
	if (!rates.success) {
		throw new Error(
			`STRIPE_TOP_UP_RATES must be a JSON object of each currency to the credits its smallest unit buys, like {"usd": 1}: ${z.prettifyError(rates.error)}`
		);
	}
	return rates.data;
}

function jsonOf(text: string): unknown {
	try {
		return JSON.parse(text);
	} catch (failure) {
		if (failure instanceof SyntaxError) return undefined;
		throw failure;
	}
}
