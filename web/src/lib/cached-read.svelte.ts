import { isSupabaseConfigured, supabase } from '$lib/supabase';

type Forgettable = { forget(): void };

const everyCachedRead = new Set<Forgettable>();
let accountGeneration = 0;
let isWatchingTheAccount = false;
let watchedAccountID: string | null = null;

function watchTheSignedInAccount(): void {
	if (isWatchingTheAccount || !isSupabaseConfigured()) return;
	isWatchingTheAccount = true;
	supabase().auth.onAuthStateChange((event, session) => {
		const accountID = session?.user.id ?? '';
		const isAnotherAccount = watchedAccountID !== null && watchedAccountID !== accountID;
		watchedAccountID = accountID;
		if (event === 'SIGNED_OUT' || isAnotherAccount) forgetEveryCachedRead();
	});
}

export class CachedRead<Value> {
	value = $state<Value | undefined>(undefined);
	readonly #lookUp: () => Promise<Value>;
	#reading: Promise<Value> | null = null;

	constructor(lookUp: () => Promise<Value>) {
		this.#lookUp = lookUp;
		everyCachedRead.add(this);
	}

	read(): Promise<Value> {
		return this.#reading ?? this.refresh();
	}

	refresh(): Promise<Value> {
		watchTheSignedInAccount();
		const startedGeneration = accountGeneration;
		const reading: Promise<Value> = this.#lookUp()
			.then((value) => {
				if (startedGeneration === accountGeneration && this.#reading === reading) this.value = value;
				return value;
			})
			.finally(() => {
				if (this.#reading === reading) this.#reading = null;
			});
		this.#reading = reading;
		return reading;
	}

	remember(value: Value): void {
		this.value = value;
	}

	forget(): void {
		this.value = undefined;
		this.#reading = null;
	}
}

export function forgetEveryCachedRead(): void {
	accountGeneration += 1;
	for (const cachedRead of everyCachedRead) cachedRead.forget();
}
