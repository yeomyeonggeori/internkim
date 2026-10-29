import type { CurrencyCatalogueEntry } from '$lib/currency/currency-catalogue';
export type ExchangeRate = { base: string; quote: string; rate: number; asOf: string };
export type ExchangeRateProvider = {
	supportedCurrencies(): Promise<CurrencyCatalogueEntry[]>;
	latestRate(base: string, quote: string): Promise<ExchangeRate>;
};

const providerName = 'Frankfurter';
export const frankfurterURL = 'https://api.frankfurter.dev/v2';
const cacheLifetimeInMilliseconds = 12 * 60 * 60 * 1000;
const rateCacheLifetimeInMilliseconds = 60 * 60 * 1000;
const defaultRequestTimeoutInMilliseconds = 5000;

type SupportedCurrenciesCache = {
	currencies: CurrencyCatalogueEntry[];
	fetchedAtInMilliseconds: number;
};

type RateCache = {
	rate: ExchangeRate;
	fetchedAtInMilliseconds: number;
};

type ProviderConnection = {
	fetchImplementation: typeof fetch;
	baseURL: string;
	requestTimeoutInMilliseconds: number;
};

type ProviderCaches = {
	supportedCurrencies: SupportedCurrenciesCache | null;
	ratesByPairKey: Map<string, RateCache>;
};

function pairKeyOf(base: string, quote: string): string {
	return `${base}:${quote}`;
}

function parsedCurrenciesPayload(payload: unknown, attemptedPath: string): CurrencyCatalogueEntry[] {
	if (!Array.isArray(payload)) {
		throw new Error(`${providerName} returned a malformed currency list from ${attemptedPath}: expected a JSON array`);
	}
	return payload.map((entry, index) => {
		if (typeof entry !== 'object' || entry === null) {
			throw new Error(
				`${providerName} returned a malformed currency list from ${attemptedPath}: entry ${index} is not an object`
			);
		}
		const record = entry as Record<string, unknown>;
		if (typeof record.iso_code !== 'string' || record.iso_code === '') {
			throw new Error(
				`${providerName} returned a malformed currency list from ${attemptedPath}: entry ${index} has no iso_code`
			);
		}
		if (typeof record.name !== 'string' || record.name === '') {
			throw new Error(
				`${providerName} returned a malformed currency list from ${attemptedPath}: ${record.iso_code} has no name`
			);
		}
		return { code: record.iso_code, name: record.name };
	});
}

function parsedLatestPayload(
	payload: unknown,
	quote: string,
	attemptedPath: string
): { base: string; date: string; rate: number } {
	if (!Array.isArray(payload)) {
		throw new Error(`${providerName} returned a malformed rate response from ${attemptedPath}: expected a JSON array`);
	}
	const quoted = payload.find(
		(entry) => typeof entry === 'object' && entry !== null && (entry as Record<string, unknown>).quote === quote
	);
	if (!quoted) {
		throw new Error(`${providerName} returned a malformed rate response from ${attemptedPath}: no rate for ${quote}`);
	}
	const record = quoted as Record<string, unknown>;
	if (typeof record.base !== 'string' || typeof record.date !== 'string') {
		throw new Error(`${providerName} returned a malformed rate response from ${attemptedPath}: missing base or date`);
	}
	if (typeof record.rate !== 'number' || !Number.isFinite(record.rate)) {
		throw new Error(
			`${providerName} returned a malformed rate response from ${attemptedPath}: rate for ${quote} is not a finite number`
		);
	}
	return { base: record.base, date: record.date, rate: record.rate };
}

function pathOf(url: URL): string {
	return `${url.pathname}${url.search}`;
}

async function fetchJSON(connection: ProviderConnection, url: URL): Promise<unknown> {
	const { fetchImplementation, requestTimeoutInMilliseconds } = connection;
	const attemptedPath = pathOf(url);
	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), requestTimeoutInMilliseconds);
	try {
		let response: Response;
		try {
			response = await fetchImplementation(url, { signal: controller.signal });
		} catch (cause) {
			const causeMessage = cause instanceof Error ? cause.message : String(cause);
			throw new Error(`${providerName} request to ${attemptedPath} failed: ${causeMessage}`);
		}
		if (!response.ok) {
			throw new Error(`${providerName} request to ${attemptedPath} failed with status ${response.status}`);
		}
		return await response.json();
	} catch (cause) {
		if (controller.signal.aborted) {
			throw new Error(`${providerName} request to ${attemptedPath} timed out after ${requestTimeoutInMilliseconds}ms`);
		}
		throw cause;
	} finally {
		clearTimeout(timeout);
	}
}

async function fetchedSupportedCurrencies(connection: ProviderConnection): Promise<CurrencyCatalogueEntry[]> {
	const currenciesURL = new URL(`${connection.baseURL}/currencies`);
	const payload = await fetchJSON(connection, currenciesURL);
	const currencies = parsedCurrenciesPayload(payload, pathOf(currenciesURL));
	return currencies.toSorted((left, right) => left.code.localeCompare(right.code));
}

async function cachedSupportedCurrencies(
	caches: ProviderCaches,
	connection: ProviderConnection,
	nowInMilliseconds: number
): Promise<CurrencyCatalogueEntry[]> {
	const cache = caches.supportedCurrencies;
	if (cache && nowInMilliseconds - cache.fetchedAtInMilliseconds < cacheLifetimeInMilliseconds) {
		return cache.currencies;
	}

	try {
		const currencies = await fetchedSupportedCurrencies(connection);
		caches.supportedCurrencies = { currencies, fetchedAtInMilliseconds: nowInMilliseconds };
		return currencies;
	} catch (cause) {
		if (cache) return cache.currencies;
		throw cause;
	}
}

function todayInUTC(nowInMilliseconds: number): string {
	return new Date(nowInMilliseconds).toISOString().slice(0, 10);
}

async function fetchedLatestRate(
	connection: ProviderConnection,
	base: string,
	quote: string,
	nowInMilliseconds: number
): Promise<ExchangeRate> {
	if (base === quote) return { base, quote, rate: 1, asOf: todayInUTC(nowInMilliseconds) };

	const latestURL = new URL(`${connection.baseURL}/rates`);
	latestURL.searchParams.set('base', base);
	latestURL.searchParams.set('quotes', quote);

	const payload = await fetchJSON(connection, latestURL);
	const parsed = parsedLatestPayload(payload, quote, pathOf(latestURL));
	return { base: parsed.base, quote, rate: parsed.rate, asOf: parsed.date };
}

async function cachedLatestRate(
	caches: ProviderCaches,
	connection: ProviderConnection,
	base: string,
	quote: string,
	nowInMilliseconds: number
): Promise<ExchangeRate> {
	if (base === quote) return fetchedLatestRate(connection, base, quote, nowInMilliseconds);

	const pairKey = pairKeyOf(base, quote);
	const cache = caches.ratesByPairKey.get(pairKey);
	if (cache && nowInMilliseconds - cache.fetchedAtInMilliseconds < rateCacheLifetimeInMilliseconds) {
		return cache.rate;
	}

	try {
		const rate = await fetchedLatestRate(connection, base, quote, nowInMilliseconds);
		caches.ratesByPairKey.set(pairKey, { rate, fetchedAtInMilliseconds: nowInMilliseconds });
		return rate;
	} catch (cause) {
		if (cache) return cache.rate;
		throw cause;
	}
}

export function frankfurterProvider(options?: {
	baseURL?: string;
	fetch?: typeof fetch;
	now?: () => number;
	requestTimeoutInMilliseconds?: number;
}): ExchangeRateProvider {
	const connection: ProviderConnection = {
		fetchImplementation: options?.fetch ?? fetch,
		baseURL: options?.baseURL ?? frankfurterURL,
		requestTimeoutInMilliseconds: options?.requestTimeoutInMilliseconds ?? defaultRequestTimeoutInMilliseconds
	};
	const now = options?.now ?? Date.now;
	const caches: ProviderCaches = { supportedCurrencies: null, ratesByPairKey: new Map() };
	return {
		supportedCurrencies: () => cachedSupportedCurrencies(caches, connection, now()),
		latestRate: (base, quote) => cachedLatestRate(caches, connection, base, quote, now())
	};
}
