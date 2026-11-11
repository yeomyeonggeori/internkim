import type { CompanyInvoice } from '$lib/company/invoice-contract';
import { formatMoney, fractionDigitsOf } from '$lib/money';

export function creditCount(credits: number, locale: string): string {
	return credits.toLocaleString(locale);
}

export function billingDay(isoDate: string | null, locale: string): string {
	if (!isoDate) return '';
	return new Date(isoDate).toLocaleDateString(locale, { month: 'long', day: 'numeric' });
}

export function filled(template: string, values: Record<string, string>): string {
	return Object.entries(values).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, value), template);
}

export function invoiceDay(isoDate: string, locale: string): string {
	return new Date(isoDate).toLocaleDateString(locale, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function invoiceAmount(invoice: Pick<CompanyInvoice, 'amount' | 'currency'>, locale: string): string {
	const currency = invoice.currency.toUpperCase();
	return formatMoney({ amount: invoice.amount / 10 ** fractionDigitsOf(currency), currency }, locale);
}
