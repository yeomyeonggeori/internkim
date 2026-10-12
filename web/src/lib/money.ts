import { z } from 'zod';

export const moneySchema = z.strictObject({
	amount: z.number().describe('The amount in the currency\'s own unit, e.g. 500000000 for ₩500,000,000 or 10.5 for $10.50.'),
	currency: z.string().regex(/^[A-Z]{3}$/).describe('ISO 4217 currency code, e.g. KRW or USD.')
});

export type Money = z.infer<typeof moneySchema>;

export function formatMoney(money: Money, locale: string): string {
	return new Intl.NumberFormat(locale, { style: 'currency', currency: money.currency }).format(money.amount);
}

export function formatCompactMoney(money: Money, locale: string): string {
	return new Intl.NumberFormat(locale, {
		style: 'currency', currency: money.currency, currencyDisplay: 'narrowSymbol', notation: 'compact', maximumFractionDigits: 1,
		useGrouping: 'always'
	}).format(money.amount);
}

export function fractionDigitsOf(currency: string): number {
	return new Intl.NumberFormat('en', { style: 'currency', currency }).resolvedOptions().maximumFractionDigits ?? 0;
}

export function roundedMoney(amount: number, currency: string): Money {
	const scale = 10 ** fractionDigitsOf(currency);
	return { amount: Math.round(amount * scale) / scale, currency };
}
