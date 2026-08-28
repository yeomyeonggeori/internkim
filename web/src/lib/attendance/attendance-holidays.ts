import { supabase } from '$lib/supabase';

export type AttendanceHolidayRange = {
	from: string;
	to: string;
};

export type NationalHolidayReader = (range: AttendanceHolidayRange) => Promise<string[]>;

const holidaysByRangeKey = new Map<string, Promise<string[]>>();

function rangeKeyOf(range: AttendanceHolidayRange): string {
	return `${range.from}:${range.to}`;
}

async function signedInHeaders(): Promise<Record<string, string>> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');
	return { Authorization: `Bearer ${accessToken}` };
}

export async function nationalHolidayDates(range: AttendanceHolidayRange): Promise<string[]> {
	const response = await fetch(
		`/api/attendance/national-holidays?from=${range.from}&to=${range.to}`,
		{ headers: await signedInHeaders() }
	);
	if (!response.ok) {
		throw new Error(
			(await response.text()).trim() || `the holiday source returned ${response.status}`
		);
	}
	const answer = (await response.json()) as { dates?: unknown };
	if (!Array.isArray(answer.dates) || !answer.dates.every((date) => typeof date === 'string')) {
		throw new Error('the holiday source returned a malformed date list');
	}
	return answer.dates;
}

export function rememberedNationalHolidayDates(range: AttendanceHolidayRange): Promise<string[]> {
	const rangeKey = rangeKeyOf(range);
	const remembered = holidaysByRangeKey.get(rangeKey);
	if (remembered) return remembered;
	const asked = nationalHolidayDates(range).catch((cause) => {
		holidaysByRangeKey.delete(rangeKey);
		throw cause;
	});
	holidaysByRangeKey.set(rangeKey, asked);
	return asked;
}

export async function attendanceHolidayDates(
	range: AttendanceHolidayRange,
	companyHolidayDates: string[],
	readNationalHolidays: NationalHolidayReader = rememberedNationalHolidayDates
): Promise<Set<string>> {
	const national = await readNationalHolidays(range);
	return new Set([...national, ...companyHolidayDates]);
}
