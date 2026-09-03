import {
	createSupabaseCompanyHoliday,
	deleteSupabaseCompanyHoliday,
	supabaseCompanyHolidays,
	updateSupabaseCompanyHoliday
} from '$lib/attendance/supabase-company-holidays';
import type { CompanyHoliday, CompanyHolidayInput, CompanyHolidaysResponse } from './admin-types';

export async function fetchCompanyHolidays(): Promise<CompanyHolidaysResponse> {
	return { holidays: await supabaseCompanyHolidays() };
}

export function createCompanyHoliday(input: CompanyHolidayInput): Promise<CompanyHoliday> {
	return createSupabaseCompanyHoliday(input);
}

export function updateCompanyHoliday(
	holidayID: string,
	input: CompanyHolidayInput
): Promise<CompanyHoliday> {
	return updateSupabaseCompanyHoliday(holidayID, input);
}

export function deleteCompanyHoliday(holidayID: string): Promise<void> {
	return deleteSupabaseCompanyHoliday(holidayID);
}
