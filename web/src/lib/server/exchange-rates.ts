export type SupportedCurrency = { code: string; name: string; minorUnitDigits: number };
export type ExchangeRate = { base: string; quote: string; rate: number; asOf: string };
export type ExchangeRateProvider = {
	supportedCurrencies(): Promise<SupportedCurrency[]>;
	latestRate(base: string, quote: string): Promise<ExchangeRate>;
};

const providerName = 'Frankfurter';
const frankfurterBaseURL = 'https://api.frankfurter.dev/v2';
const cacheLifetimeInMilliseconds = 12 * 60 * 60 * 1000;
const rateCacheLifetimeInMilliseconds = 60 * 60 * 1000;
const defaultRequestTimeoutInMilliseconds = 5000;

const zeroDecimalCurrencyCodes = new Set([
	'BIF',
	'CLP',
	'DJF',
	'GNF',
	'ISK',
	'JPY',
	'KMF',
	'KRW',
	'PYG',
	'RWF',
	'UGX',
	'VND',
	'VUV',
	'XAF',
	'XOF',
	'XPF'
]);

const threeDecimalCurrencyCodes = new Set(['BHD', 'IQD', 'JOD', 'KWD', 'LYD', 'OMR', 'TND']);

export function minorUnitDigitsOf(code: string): number {
	if (zeroDecimalCurrencyCodes.has(code)) return 0;
	if (threeDecimalCurrencyCodes.has(code)) return 3;
	return 2;
}

export function convertMinorAmount(amountMinor: number, rate: number, fromDigits: number, toDigits: number): number {
	const scaledAmount = amountMinor * rate * 10 ** (toDigits - fromDigits);
	return Math.round(scaledAmount);
}

type SupportedCurrenciesCache = {
	currencies: SupportedCurrency[];
	fetchedAtInMilliseconds: number;
};

type RateCache = {
	rate: ExchangeRate;
	fetchedAtInMilliseconds: number;
};

type ProviderCaches = {
	supportedCurrencies: SupportedCurrenciesCache | null;
	ratesByPairKey: Map<string, RateCache>;
};

function pairKeyOf(base: string, quote: string): string {
	return `${base}:${quote}`;
}

type FrankfurterCurrency = { code: string; name: string };

function parsedCurrenciesPayload(payload: unknown, attemptedPath: string): FrankfurterCurrency[] {
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

async function fetchJSON(
	fetchImplementation: typeof fetch,
	url: URL,
	requestTimeoutInMilliseconds: number
): Promise<unknown> {
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

async function fetchedSupportedCurrencies(
	fetchImplementation: typeof fetch,
	requestTimeoutInMilliseconds: number
): Promise<SupportedCurrency[]> {
	const currenciesURL = new URL(`${frankfurterBaseURL}/currencies`);
	const payload = await fetchJSON(fetchImplementation, currenciesURL, requestTimeoutInMilliseconds);
	const currencies = parsedCurrenciesPayload(payload, pathOf(currenciesURL));
	return currencies.map(({ code, name }) => ({
		code,
		name,
		minorUnitDigits: minorUnitDigitsOf(code)
	}));
}

async function cachedSupportedCurrencies(
	caches: ProviderCaches,
	fetchImplementation: typeof fetch,
	nowInMilliseconds: number,
	requestTimeoutInMilliseconds: number
): Promise<SupportedCurrency[]> {
	const cache = caches.supportedCurrencies;
	if (cache && nowInMilliseconds - cache.fetchedAtInMilliseconds < cacheLifetimeInMilliseconds) {
		return cache.currencies;
	}

	try {
		const currencies = await fetchedSupportedCurrencies(fetchImplementation, requestTimeoutInMilliseconds);
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
	fetchImplementation: typeof fetch,
	base: string,
	quote: string,
	nowInMilliseconds: number,
	requestTimeoutInMilliseconds: number
): Promise<ExchangeRate> {
	if (base === quote) return { base, quote, rate: 1, asOf: todayInUTC(nowInMilliseconds) };

	const latestURL = new URL(`${frankfurterBaseURL}/rates`);
	latestURL.searchParams.set('base', base);
	latestURL.searchParams.set('quotes', quote);

	const payload = await fetchJSON(fetchImplementation, latestURL, requestTimeoutInMilliseconds);
	const parsed = parsedLatestPayload(payload, quote, pathOf(latestURL));
	return { base: parsed.base, quote, rate: parsed.rate, asOf: parsed.date };
}

async function cachedLatestRate(
	caches: ProviderCaches,
	fetchImplementation: typeof fetch,
	base: string,
	quote: string,
	nowInMilliseconds: number,
	requestTimeoutInMilliseconds: number
): Promise<ExchangeRate> {
	if (base === quote) {
		return fetchedLatestRate(fetchImplementation, base, quote, nowInMilliseconds, requestTimeoutInMilliseconds);
	}

	const pairKey = pairKeyOf(base, quote);
	const cache = caches.ratesByPairKey.get(pairKey);
	if (cache && nowInMilliseconds - cache.fetchedAtInMilliseconds < rateCacheLifetimeInMilliseconds) {
		return cache.rate;
	}

	try {
		const rate = await fetchedLatestRate(fetchImplementation, base, quote, nowInMilliseconds, requestTimeoutInMilliseconds);
		caches.ratesByPairKey.set(pairKey, { rate, fetchedAtInMilliseconds: nowInMilliseconds });
		return rate;
	} catch (cause) {
		if (cache) return cache.rate;
		throw cause;
	}
}

export function frankfurterProvider(options?: {
	fetch?: typeof fetch;
	now?: () => number;
	requestTimeoutInMilliseconds?: number;
}): ExchangeRateProvider {
	const fetchImplementation = options?.fetch ?? fetch;
	const now = options?.now ?? Date.now;
	const requestTimeoutInMilliseconds = options?.requestTimeoutInMilliseconds ?? defaultRequestTimeoutInMilliseconds;
	const caches: ProviderCaches = { supportedCurrencies: null, ratesByPairKey: new Map() };
	return {
		supportedCurrencies: () => cachedSupportedCurrencies(caches, fetchImplementation, now(), requestTimeoutInMilliseconds),
		latestRate: (base, quote) => cachedLatestRate(caches, fetchImplementation, base, quote, now(), requestTimeoutInMilliseconds)
	};
}
