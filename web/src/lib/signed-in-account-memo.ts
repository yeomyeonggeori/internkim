import type { SignedInMember } from '$lib/supabase-session';
import { invalidateMessengerCacheScope } from '$lib/messenger/cache-scope';

type RememberedLookup<Value> = { accountID: string; value: Promise<Value> };

class AccountMemo<Value> {
	#remembered: RememberedLookup<Value> | null = null;
	readonly #isWorthKeeping: (value: Value) => boolean;

	constructor(isWorthKeeping: (value: Value) => boolean) {
		this.#isWorthKeeping = isWorthKeeping;
	}

	of(accountID: string, lookUp: () => Promise<Value>): Promise<Value> {
		if (this.#remembered?.accountID === accountID) return this.#remembered.value;
		const lookup = { accountID, value: lookUp() };
		this.#remembered = lookup;
		lookup.value.then(
			(value) => {
				if (!this.#isWorthKeeping(value)) this.#forgetLookup(lookup);
			},
			() => this.#forgetLookup(lookup)
		);
		return lookup.value;
	}

	forget(): void {
		this.#remembered = null;
	}

	#forgetLookup(lookup: RememberedLookup<Value>): void {
		if (this.#remembered === lookup) this.#remembered = null;
	}
}

export const companyMembership = new AccountMemo<boolean>((belongs) => belongs);
export const signedInMember = new AccountMemo<SignedInMember>((member) => member.memberID !== '');

export function forgetSignedInAccount(forgetStoredMessenger = false): void {
	companyMembership.forget();
	signedInMember.forget();
	invalidateMessengerCacheScope(forgetStoredMessenger);
}
