import { supabase } from '$lib/supabase';
import type {
	AttendanceLeavePolicy,
	AttendanceWorkPolicyRevision,
	CompanyHoliday
} from '../../routes/admin/admin-types';

export type CompanyRules = {
	attendanceWorkPolicy?: AttendanceWorkPolicyRevision;
	attendanceLeavePolicy?: AttendanceLeavePolicy;
	companyHolidays?: CompanyHoliday[];
};

export type CompanySettings = {
	rules: CompanyRules;
	timeZone: string;
	leaveDays: number | null;
};

type CompanySettingsRow = {
	timezone: string;
	leave_days: number | null;
	rules: CompanyRules | null;
};

export async function companySettings(): Promise<CompanySettings> {
	const company = await supabase()
		.from('company')
		.select('timezone, leave_days, rules')
		.limit(1)
		.single<CompanySettingsRow>();
	if (company.error) throw new Error(company.error.message);
	return {
		rules: company.data.rules ?? {},
		timeZone: company.data.timezone,
		leaveDays: company.data.leave_days
	};
}
