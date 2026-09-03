import { invokeTool } from '$lib/public-api-call';
import type { CompanyHoliday, CompanyHolidayInput } from '../../routes/admin/admin-types';

type AnsweredHoliday = {
	holidayID: string;
	name: string;
	date: string;
	recursAnnually: boolean;
	createdAt: string | null;
	updatedAt: string | null;
};

type AnsweredHolidays = { count: number; year: number | null; holidays: AnsweredHoliday[] };

function companyHolidayOf(answered: AnsweredHoliday): CompanyHoliday {
	return {
		id: answered.holidayID,
		title: answered.name,
		date: answered.date,
		recursAnnually: answered.recursAnnually,
		createdAt: answered.createdAt ?? '',
		updatedAt: answered.updatedAt ?? ''
	};
}

export async function supabaseCompanyHolidays(): Promise<CompanyHoliday[]> {
	const answered = await invokeTool<AnsweredHolidays>('company_holiday_list', {});
	return answered.holidays.map(companyHolidayOf);
}

export async function createSupabaseCompanyHoliday(
	input: CompanyHolidayInput
): Promise<CompanyHoliday> {
	const added = await invokeTool<AnsweredHoliday>('company_holiday_add', {
		date: input.date,
		name: input.title,
		recursAnnually: input.recursAnnually
	});
	return companyHolidayOf(added);
}

export async function updateSupabaseCompanyHoliday(
	holidayID: string,
	input: CompanyHolidayInput
): Promise<CompanyHoliday> {
	const written = await invokeTool<AnsweredHoliday>('company_holiday_update', {
		holidayHint: holidayID,
		date: input.date,
		name: input.title,
		recursAnnually: input.recursAnnually
	});
	return companyHolidayOf(written);
}

export async function deleteSupabaseCompanyHoliday(holidayID: string): Promise<void> {
	await invokeTool<AnsweredHoliday>('company_holiday_delete', { holidayHint: holidayID });
}
