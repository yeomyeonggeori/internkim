export type CompanyDateTime = {
	year: number;
	month: number;
	day: number;
	hour: number;
	minute: number;
	second: number;
};

export function companyDateTimeOf(instant: Date, timeZone: string): CompanyDateTime {
	const parts = dateTimeFormatterFor(timeZone).formatToParts(instant);
	const numberOf = (type: Intl.DateTimeFormatPartTypes): number =>
		Number(parts.find((part) => part.type === type)?.value ?? Number.NaN);
	return {
		year: numberOf('year'),
		month: numberOf('month'),
		day: numberOf('day'),
		hour: numberOf('hour'),
		minute: numberOf('minute'),
		second: numberOf('second')
	};
}

export function companyDateOf(instant: Date, timeZone: string): string {
	return companyDateString(companyDateTimeOf(instant, timeZone));
}

export function companyTimeOf(instant: Date, timeZone: string): string {
	return companyTimeString(companyDateTimeOf(instant, timeZone));
}

export function companyDateString(moment: CompanyDateTime): string {
	return `${String(moment.year).padStart(4, '0')}-${twoDigits(moment.month)}-${twoDigits(moment.day)}`;
}

export function companyTimeString(moment: CompanyDateTime): string {
	return `${twoDigits(moment.hour)}:${twoDigits(moment.minute)}`;
}

export function companyInstantOf(day: string, time: string, timeZone: string): string | undefined {
	const expectedEpoch = new Date(`${day}T${time}:00Z`).getTime();
	if (Number.isNaN(expectedEpoch)) return undefined;

	let resolvedEpoch = expectedEpoch;
	for (let attempt = 0; attempt < 3; attempt += 1) {
		const represented = companyDateTimeOf(new Date(resolvedEpoch), timeZone);
		const correction = expectedEpoch - epochOfCompanyDateTime(represented);
		if (correction === 0) return new Date(resolvedEpoch).toISOString();
		resolvedEpoch += correction;
	}

	const resolved = new Date(resolvedEpoch);
	const moment = companyDateTimeOf(resolved, timeZone);
	if (companyDateString(moment) !== day || companyTimeString(moment) !== time) return undefined;
	return resolved.toISOString();
}

export function isValidTimeZone(timeZone: string): boolean {
	try {
		new Intl.DateTimeFormat('en-US', { timeZone });
		return true;
	} catch (error) {
		if (error instanceof RangeError) return false;
		throw error;
	}
}

function epochOfCompanyDateTime(moment: CompanyDateTime): number {
	return Date.UTC(moment.year, moment.month - 1, moment.day, moment.hour, moment.minute, moment.second);
}

const dateTimeFormatterByTimeZone = new Map<string, Intl.DateTimeFormat>();

function dateTimeFormatterFor(timeZone: string): Intl.DateTimeFormat {
	const formatter = dateTimeFormatterByTimeZone.get(timeZone);
	if (formatter) return formatter;
	const createdFormatter = new Intl.DateTimeFormat('en-US', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit',
		hourCycle: 'h23'
	});
	dateTimeFormatterByTimeZone.set(timeZone, createdFormatter);
	return createdFormatter;
}

function twoDigits(value: number): string {
	return String(value).padStart(2, '0');
}
