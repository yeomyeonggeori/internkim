import type { Money } from '$lib/money';
import { minorUnitDigitsOf } from './exchange-rates';

export function moneyFromMinor(amountMinor: number, currency: string): Money {
	return { amount: amountMinor / 10 ** minorUnitDigitsOf(currency), currency };
}

export function storedMoneyOf(amountMinor: number | null, currency: string | null): Money | null {
	if (amountMinor === null || !currency) return null;
	return moneyFromMinor(amountMinor, currency);
}

export function minorOfMoney(money: Money): number {
	return Math.round(money.amount * 10 ** minorUnitDigitsOf(money.currency));
}
