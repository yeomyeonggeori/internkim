import { loadConvertedAmount } from '$lib/currency/converted-amount';
import { moneySchema, roundedMoney, type Money } from '$lib/money';
import type { DataRoomCompany } from './company';

export type ExchangeRates = Map<string, { rate: number; asOf: string }>;

export function pairOf(from: string, to: string): string {
	return `${from}:${to}`;
}

export function recordedCurrencies(company: DataRoomCompany): string[] {
	const amounts = [
		company.profile.capital,
		...company.metrics.map((metric) => metric.amount),
		...company.records.flatMap((record) => [record.amount, record.committed, ...moneyAttributes(record.attributes)])
	];
	return [...new Set(amounts.flatMap((money) => (money ? [money.currency] : [])))];
}

export async function loadExchangeRates(currencies: string[], to: string): Promise<ExchangeRates> {
	const loaded = await Promise.all(
		currencies.filter((from) => from !== to).map(async (from) => [from, await loadConvertedAmount({ amount: 1, currency: from }, to)] as const)
	);
	return new Map(loaded.flatMap(([from, converted]) => (converted ? [[pairOf(from, to), { rate: converted.rate, asOf: converted.asOf }] as const] : [])));
}

export function companyIn(company: DataRoomCompany, to: string, rates: ExchangeRates): DataRoomCompany {
	const convert = (money: Money): Money => {
		const rate = money.currency === to ? 1 : rates.get(pairOf(money.currency, to))?.rate;
		return rate === undefined ? money : roundedMoney(money.amount * rate, to);
	};
	const converted = <Shape extends { amount?: Money }>(holder: Shape): Shape => (holder.amount ? { ...holder, amount: convert(holder.amount) } : holder);
	return {
		...company,
		profile: company.profile.capital ? { ...company.profile, capital: convert(company.profile.capital) } : company.profile,
		metrics: company.metrics.map(converted),
		records: company.records.map((record) => {
			const withAmount = { ...converted(record), attributes: attributesIn(record.attributes, convert) };
			return record.committed ? { ...withAmount, committed: convert(record.committed) } : withAmount;
		})
	};
}

function moneyAttributes(attributes: Record<string, unknown>): Money[] {
	return Object.values(attributes).flatMap((value) => {
		const money = moneySchema.safeParse(value);
		return money.success ? [money.data] : [];
	});
}

function attributesIn(attributes: Record<string, unknown>, convert: (money: Money) => Money): Record<string, unknown> {
	return Object.fromEntries(
		Object.entries(attributes).map(([key, value]) => {
			const money = moneySchema.safeParse(value);
			return [key, money.success ? convert(money.data) : value];
		})
	);
}
