export type StripeEventOutcome = 'applied' | 'ignored';

export type UnitAmountOfPrice = (priceID: string) => Promise<number>;

export type InvoicePaidBy = (paymentIntentID: string) => Promise<unknown>;

export type StripeLookups = { unitAmountOfPrice: UnitAmountOfPrice; invoicePaidBy: InvoicePaidBy };

export class StripeEventRefused extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'StripeEventRefused';
	}
}
