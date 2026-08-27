import { supabase } from '$lib/supabase';
import { companySettings } from '$lib/company/company-settings';
import type { CompanyHoliday, CompanyHolidayInput } from '../../routes/admin/admin-types';

export async function supabaseCompanyHolidays(): Promise<CompanyHoliday[]> {
	return (await companySettings()).rules.companyHolidays ?? [];
}

export async function createSupabaseCompanyHoliday(
	input: CompanyHolidayInput
): Promise<CompanyHoliday> {
	const now = new Date().toISOString();
	const created: CompanyHoliday = {
		id: `company-holiday-${crypto.randomUUID().replaceAll('-', '')}`,
		...input,
		createdAt: now,
		updatedAt: now
	};
	await saveCompanyHolidays([...(await supabaseCompanyHolidays()), created]);
	return created;
}

export async function updateSupabaseCompanyHoliday(
	holidayID: string,
	input: CompanyHolidayInput
): Promise<CompanyHoliday> {
	const holidays = await supabaseCompanyHolidays();
	const existing = holidays.find((holiday) => holiday.id === holidayID);
	if (!existing) throw new Error(`company holiday ${holidayID} no longer exists`);
	const updated: CompanyHoliday = { ...existing, ...input, updatedAt: new Date().toISOString() };
	await saveCompanyHolidays(
		holidays.map((holiday) => (holiday.id === holidayID ? updated : holiday))
	);
	return updated;
}

export async function deleteSupabaseCompanyHoliday(holidayID: string): Promise<void> {
	const holidays = await supabaseCompanyHolidays();
	await saveCompanyHolidays(holidays.filter((holiday) => holiday.id !== holidayID));
}

async function saveCompanyHolidays(holidays: CompanyHoliday[]): Promise<void> {
	const saved = await supabase().rpc('company_holidays_save', { target_holidays: holidays });
	if (saved.error) throw new Error(saved.error.message);
}
