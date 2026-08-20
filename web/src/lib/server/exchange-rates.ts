export type SupportedCurrency = { code: string; name: string; minorUnitDigits: number };
export type ExchangeRate = { base: string; quote: string; rate: number; asOf: string };
export type ExchangeRateProvider = {
	supportedCurrencies(): Promise<SupportedCurrency[]>;
	latestRate(base: string, quote: string): Promise<ExchangeRate>;
};

const providerName = 'Frankfurter';
const frankfurterBaseURL = 'https://api.frankfurter.dev/v1';
const cacheLifetimeInMilliseconds = 12 * 60 * 60 * 1000;

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

let supportedCurrenciesCache: SupportedCurrenciesCache | null = null;

type FrankfurterCurrenciesPayload = Record<string, string>;

function parsedCurrenciesPayload(payload: unknown, attemptedPath: string): FrankfurterCurrenciesPayload {
	if (typeof payload !== 'object' || payload === null || Array.isArray(payload)) {
		throw new Error(`${providerName} returned a malformed currency list from ${attemptedPath}: expected a JSON object`);
	}
	const record = payload as Record<string, unknown>;
	const invalidEntry = Object.entries(record).find(([, name]) => typeof name !== 'string');
	if (invalidEntry) {
		throw new Error(`${providerName} returned a malformed currency list from ${attemptedPath}: ${invalidEntry[0]} has no name`);
	}
	return record as FrankfurterCurrenciesPayload;
}

function parsedLatestPayload(
	payload: unknown,
	quote: string,
	attemptedPath: string
): { base: string; date: string; rate: number } {
	if (typeof payload !== 'object' || payload === null) {
		throw new Error(`${providerName} returned a malformed rate response from ${attemptedPath}: expected a JSON object`);
	}
	const record = payload as Record<string, unknown>;
	if (typeof record.base !== 'string' || typeof record.date !== 'string') {
		throw new Error(`${providerName} returned a malformed rate response from ${attemptedPath}: missing base or date`);
	}
	if (typeof record.rates !== 'object' || record.rates === null || Array.isArray(record.rates)) {
		throw new Error(`${providerName} returned a malformed rate response from ${attemptedPath}: missing rates`);
	}
	const rates = record.rates as Record<string, unknown>;
	if (!(quote in rates)) {
		throw new Error(`${providerName} returned a malformed rate response from ${attemptedPath}: no rate for ${quote}`);
	}
	const rate = rates[quote];
	if (typeof rate !== 'number' || !Number.isFinite(rate)) {
		throw new Error(
			`${providerName} returned a malformed rate response from ${attemptedPath}: rate for ${quote} is not a finite number`
		);
	}
	return { base: record.base, date: record.date, rate };
}

function pathOf(url: URL): string {
	return `${url.pathname}${url.search}`;
}

async function fetchJSON(fetchImplementation: typeof fetch, url: URL): Promise<unknown> {
	const attemptedPath = pathOf(url);
	let response: Response;
	try {
		response = await fetchImplementation(url);
	} catch (cause) {
		const causeMessage = cause instanceof Error ? cause.message : String(cause);
		throw new Error(`${providerName} request to ${attemptedPath} failed: ${causeMessage}`);
	}
	if (!response.ok) {
		throw new Error(`${providerName} request to ${attemptedPath} failed with status ${response.status}`);
	}
	return response.json();
}

async function fetchedSupportedCurrencies(fetchImplementation: typeof fetch): Promise<SupportedCurrency[]> {
	const currenciesURL = new URL(`${frankfurterBaseURL}/currencies`);
	const payload = await fetchJSON(fetchImplementation, currenciesURL);
	const currenciesByCode = parsedCurrenciesPayload(payload, pathOf(currenciesURL));
	return Object.entries(currenciesByCode).map(([code, name]) => ({
		code,
		name,
		minorUnitDigits: minorUnitDigitsOf(code)
	}));
}

async function cachedSupportedCurrencies(
	fetchImplementation: typeof fetch,
	nowInMilliseconds: number
): Promise<SupportedCurrency[]> {
	const cache = supportedCurrenciesCache;
	if (cache && nowInMilliseconds - cache.fetchedAtInMilliseconds < cacheLifetimeInMilliseconds) {
		return cache.currencies;
	}

	try {
		const currencies = await fetchedSupportedCurrencies(fetchImplementation);
		supportedCurrenciesCache = { currencies, fetchedAtInMilliseconds: nowInMilliseconds };
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
	nowInMilliseconds: number
): Promise<ExchangeRate> {
	if (base === quote) return { base, quote, rate: 1, asOf: todayInUTC(nowInMilliseconds) };

	const latestURL = new URL(`${frankfurterBaseURL}/latest`);
	latestURL.searchParams.set('base', base);
	latestURL.searchParams.set('symbols', quote);

	const payload = await fetchJSON(fetchImplementation, latestURL);
	const parsed = parsedLatestPayload(payload, quote, pathOf(latestURL));
	return { base: parsed.base, quote, rate: parsed.rate, asOf: parsed.date };
}

export function frankfurterProvider(options?: { fetch?: typeof fetch; now?: () => number }): ExchangeRateProvider {
	const fetchImplementation = options?.fetch ?? fetch;
	const now = options?.now ?? Date.now;
	return {
		supportedCurrencies: () => cachedSupportedCurrencies(fetchImplementation, now()),
		latestRate: (base, quote) => fetchedLatestRate(fetchImplementation, base, quote, now())
	};
}
