import { fractionDigitsOf, type Money } from '$lib/money';

export function storedMoneyOf(amountMinor: number | null, currency: string | null): Money | null {
	if (amountMinor === null || !currency) return null;
	return { amount: amountMinor / 10 ** fractionDigitsOf(currency), currency };
}

export function minorOfMoney(money: Money): number {
	return Math.round(money.amount * 10 ** fractionDigitsOf(money.currency));
}
