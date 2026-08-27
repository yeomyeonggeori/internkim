export type NationalHolidayProvider = {
	holidayDates(countryCode: string, year: number): Promise<string[]>;
};

const providerName = 'Nager.Date';
const nagerBaseURL = 'https://date.nager.at/api/v3';
const cacheLifetimeInMilliseconds = 12 * 60 * 60 * 1000;
const requestTimeoutInMilliseconds = 5000;
const publicHolidayType = 'Public';

type YearCache = {
	dates: string[];
	fetchedAtInMilliseconds: number;
};

function yearKeyOf(countryCode: string, year: number): string {
	return `${countryCode}:${year}`;
}

function pathOf(url: URL): string {
	return `${url.pathname}${url.search}`;
}

function isDateOnly(value: unknown): value is string {
	if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
	const parsed = new Date(`${value}T00:00:00Z`);
	return !Number.isNaN(parsed.valueOf()) && parsed.toISOString().slice(0, 10) === value;
}

function isPublicHoliday(entry: Record<string, unknown>): boolean {
	if (!('types' in entry) || entry.types === null || entry.types === undefined) return true;
	if (!Array.isArray(entry.types)) return false;
	if (entry.types.length === 0) return true;
	return entry.types.includes(publicHolidayType);
}

function parsedHolidayPayload(
	payload: unknown,
	countryCode: string,
	attemptedPath: string
): string[] {
	if (!Array.isArray(payload)) {
		throw new Error(
			`${providerName} returned a malformed holiday list from ${attemptedPath}: expected a JSON array`
		);
	}
	const dates = new Set<string>();
	for (const entry of payload) {
		if (typeof entry !== 'object' || entry === null) {
			throw new Error(
				`${providerName} returned a malformed holiday list from ${attemptedPath}: an entry is not an object`
			);
		}
		const holiday = entry as Record<string, unknown>;
		if (!isDateOnly(holiday.date)) {
			throw new Error(
				`${providerName} returned a malformed holiday list from ${attemptedPath}: an entry has no calendar date`
			);
		}
		if (
			typeof holiday.countryCode !== 'string' ||
			holiday.countryCode.trim().toUpperCase() !== countryCode
		) {
			throw new Error(
				`${providerName} returned holidays for ${String(holiday.countryCode)} from ${attemptedPath}, not ${countryCode}`
			);
		}
		if (!isPublicHoliday(holiday)) continue;
		dates.add(holiday.date);
	}
	return [...dates].sort();
}

async function fetchedOnce(fetchImplementation: typeof fetch, url: URL): Promise<Response> {
	return fetchImplementation(url, { signal: AbortSignal.timeout(requestTimeoutInMilliseconds) });
}

async function fetchJSON(fetchImplementation: typeof fetch, url: URL): Promise<unknown> {
	const attemptedPath = pathOf(url);
	let response: Response;
	try {
		response = await fetchedOnce(fetchImplementation, url);
	} catch (cause) {
		try {
			response = await fetchedOnce(fetchImplementation, url);
		} catch (retried) {
			const causeMessage = retried instanceof Error ? retried.message : String(retried);
			throw new Error(`${providerName} request to ${attemptedPath} failed: ${causeMessage}`);
		}
	}
	if (!response.ok) {
		throw new Error(
			`${providerName} request to ${attemptedPath} failed with status ${response.status}`
		);
	}
	return response.json();
}

async function fetchedHolidayDates(
	fetchImplementation: typeof fetch,
	countryCode: string,
	year: number
): Promise<string[]> {
	const holidaysURL = new URL(
		`${nagerBaseURL}/PublicHolidays/${year}/${encodeURIComponent(countryCode)}`
	);
	const payload = await fetchJSON(fetchImplementation, holidaysURL);
	return parsedHolidayPayload(payload, countryCode, pathOf(holidaysURL));
}

async function cachedHolidayDates(
	caches: Map<string, YearCache>,
	fetchImplementation: typeof fetch,
	countryCode: string,
	year: number,
	nowInMilliseconds: number
): Promise<string[]> {
	const yearKey = yearKeyOf(countryCode, year);
	const cache = caches.get(yearKey);
	if (cache && nowInMilliseconds - cache.fetchedAtInMilliseconds < cacheLifetimeInMilliseconds) {
		return cache.dates;
	}

	try {
		const dates = await fetchedHolidayDates(fetchImplementation, countryCode, year);
		caches.set(yearKey, { dates, fetchedAtInMilliseconds: nowInMilliseconds });
		return dates;
	} catch (cause) {
		if (cache) return cache.dates;
		throw cause;
	}
}

export function countryCodeOf(country: string): string {
	const code = country.trim().toUpperCase();
	if (!/^[A-Z]{2}$/.test(code)) {
		throw new Error(`a national holiday lookup needs a two-letter country code, not ${country}`);
	}
	return code;
}

export function yearsBetween(from: string, to: string): number[] {
	if (!isDateOnly(from) || !isDateOnly(to)) {
		throw new Error('a national holiday range needs two YYYY-MM-DD dates');
	}
	if (from > to) throw new Error('a national holiday range must not end before it begins');
	const firstYear = Number(from.slice(0, 4));
	const lastYear = Number(to.slice(0, 4));
	const years: number[] = [];
	for (let year = firstYear; year <= lastYear; year += 1) years.push(year);
	return years;
}

export function nagerDateProvider(options?: {
	fetch?: typeof fetch;
	now?: () => number;
}): NationalHolidayProvider {
	const fetchImplementation = options?.fetch ?? fetch;
	const now = options?.now ?? Date.now;
	const caches = new Map<string, YearCache>();
	return {
		holidayDates: (countryCode, year) =>
			cachedHolidayDates(caches, fetchImplementation, countryCodeOf(countryCode), year, now())
	};
}
