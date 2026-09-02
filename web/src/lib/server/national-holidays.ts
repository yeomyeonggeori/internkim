export type NationalHoliday = {
	date: string;
	name: string;
	localName: string;
};

export type NationalHolidayProvider = {
	holidays(countryCode: string, year: number): Promise<NationalHoliday[]>;
	holidayDates(countryCode: string, year: number): Promise<string[]>;
};

const providerName = 'Nager.Date';
const nagerBaseURL = 'https://date.nager.at/api/v3';
const cacheLifetimeInMilliseconds = 12 * 60 * 60 * 1000;
const requestTimeoutInMilliseconds = 5000;
const publicHolidayType = 'Public';

type YearCache = {
	holidays: NationalHoliday[];
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

function namedAs(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

function orderedHolidays(holidays: NationalHoliday[]): NationalHoliday[] {
	return [...holidays].sort((left, right) => {
		if (left.date !== right.date) return left.date < right.date ? -1 : 1;
		if (left.name !== right.name) return left.name < right.name ? -1 : 1;
		return left.localName < right.localName ? -1 : left.localName > right.localName ? 1 : 0;
	});
}

function parsedHolidayPayload(
	payload: unknown,
	countryCode: string,
	attemptedPath: string
): NationalHoliday[] {
	if (!Array.isArray(payload)) {
		throw new Error(
			`${providerName} returned a malformed holiday list from ${attemptedPath}: expected a JSON array`
		);
	}
	const found = new Map<string, NationalHoliday>();
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
		const name = namedAs(holiday.name);
		const localName = namedAs(holiday.localName);
		if (!name && !localName) {
			throw new Error(
				`${providerName} returned a malformed holiday list from ${attemptedPath}: the holiday on ${holiday.date} has no name`
			);
		}
		if (!isPublicHoliday(holiday)) continue;
		found.set(`${holiday.date}|${name}|${localName}`, { date: holiday.date, name, localName });
	}
	return orderedHolidays([...found.values()]);
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

async function fetchedHolidays(
	fetchImplementation: typeof fetch,
	countryCode: string,
	year: number
): Promise<NationalHoliday[]> {
	const holidaysURL = new URL(
		`${nagerBaseURL}/PublicHolidays/${year}/${encodeURIComponent(countryCode)}`
	);
	const payload = await fetchJSON(fetchImplementation, holidaysURL);
	return parsedHolidayPayload(payload, countryCode, pathOf(holidaysURL));
}

async function cachedHolidays(
	caches: Map<string, YearCache>,
	fetchImplementation: typeof fetch,
	countryCode: string,
	year: number,
	nowInMilliseconds: number
): Promise<NationalHoliday[]> {
	const yearKey = yearKeyOf(countryCode, year);
	const cache = caches.get(yearKey);
	if (cache && nowInMilliseconds - cache.fetchedAtInMilliseconds < cacheLifetimeInMilliseconds) {
		return cache.holidays;
	}

	try {
		const holidays = await fetchedHolidays(fetchImplementation, countryCode, year);
		caches.set(yearKey, { holidays, fetchedAtInMilliseconds: nowInMilliseconds });
		return holidays;
	} catch (cause) {
		if (cache) return cache.holidays;
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
	const holidays = (countryCode: string, year: number) =>
		cachedHolidays(caches, fetchImplementation, countryCodeOf(countryCode), year, now());
	return {
		holidays,
		holidayDates: async (countryCode, year) => [
			...new Set((await holidays(countryCode, year)).map((holiday) => holiday.date))
		]
	};
}
