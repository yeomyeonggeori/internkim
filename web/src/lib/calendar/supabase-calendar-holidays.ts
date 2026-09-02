import { companyDateOf } from '$lib/company-time';
import { companySettings } from '$lib/company/company-settings';
import { supabase } from '$lib/supabase';
import type { CalendarHoliday, CalendarHolidayLocale } from './holiday';

type CalendarHolidaysAnswerDocument = {
	holidays?: CalendarHoliday[];
	degraded?: boolean;
	errorCode?: string;
};

export type SupabaseCalendarHolidays = {
	holidays: CalendarHoliday[];
	degraded: boolean;
	errorCode?: string;
};

async function signedInHeaders(): Promise<Record<string, string>> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');
	return { Authorization: `Bearer ${accessToken}` };
}

export async function supabaseCalendarHolidays(
	startDate: Date,
	endDate: Date,
	locale: CalendarHolidayLocale
): Promise<SupabaseCalendarHolidays> {
	const timeZone = (await companySettings()).timeZone;
	const query = new URLSearchParams({
		from: companyDateOf(startDate, timeZone),
		to: companyDateOf(endDate, timeZone),
		locale
	});
	const response = await fetch(`/api/calendar/holidays?${query}`, {
		headers: await signedInHeaders()
	});
	if (!response.ok) {
		throw new Error(
			(await response.text()).trim() || `the holiday source returned ${response.status}`
		);
	}
	const answer = (await response.json()) as CalendarHolidaysAnswerDocument;
	return {
		holidays: answer.holidays ?? [],
		degraded: answer.degraded === true,
		errorCode: answer.errorCode
	};
}
