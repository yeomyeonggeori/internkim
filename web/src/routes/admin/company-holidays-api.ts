import { isSupabaseConfigured } from '$lib/supabase';
import {
	createSupabaseCompanyHoliday,
	deleteSupabaseCompanyHoliday,
	supabaseCompanyHolidays,
	updateSupabaseCompanyHoliday
} from '$lib/attendance/supabase-company-holidays';
import * as deviceAdminAPI from './admin-api';
import type { CompanyHoliday, CompanyHolidayInput, CompanyHolidaysResponse } from './admin-types';

export async function fetchCompanyHolidays(
	adminBaseURL: string,
	fallbackMessage: string
): Promise<CompanyHolidaysResponse> {
	if (isSupabaseConfigured()) return { holidays: await supabaseCompanyHolidays() };
	return deviceAdminAPI.fetchCompanyHolidays(adminBaseURL, fallbackMessage);
}

export async function createCompanyHoliday(
	adminBaseURL: string,
	input: CompanyHolidayInput,
	fallbackMessage: string
): Promise<CompanyHoliday> {
	if (isSupabaseConfigured()) return createSupabaseCompanyHoliday(input);
	return deviceAdminAPI.createCompanyHoliday(adminBaseURL, input, fallbackMessage);
}

export async function updateCompanyHoliday(
	adminBaseURL: string,
	holidayID: string,
	input: CompanyHolidayInput,
	fallbackMessage: string
): Promise<CompanyHoliday> {
	if (isSupabaseConfigured()) return updateSupabaseCompanyHoliday(holidayID, input);
	return deviceAdminAPI.updateCompanyHoliday(adminBaseURL, holidayID, input, fallbackMessage);
}

export async function deleteCompanyHoliday(
	adminBaseURL: string,
	holidayID: string,
	fallbackMessage: string
): Promise<void> {
	if (isSupabaseConfigured()) return deleteSupabaseCompanyHoliday(holidayID);
	return deviceAdminAPI.deleteCompanyHoliday(adminBaseURL, holidayID, fallbackMessage);
}
