import { supabase } from '$lib/supabase';
import { companySettings, type CompanySettings } from '$lib/company/company-settings';
import { companyHolidayDatesInMonth } from './company-holiday-dates';
import { currentAttendanceWorkPolicy } from './current-work-policy';
import { defaultWorkPolicy, initialWorkPolicyEffectiveDate } from './work-policy-defaults';
import type {
	AttendanceWorkPolicyResponse,
	AttendanceWorkPolicyRevision
} from '../../routes/admin/admin-types';

export async function supabaseAttendanceWorkPolicy(): Promise<AttendanceWorkPolicyResponse> {
	return workPolicyResponse(await companySettings());
}

export async function saveSupabaseAttendanceWorkPolicy(
	revision: AttendanceWorkPolicyRevision
): Promise<AttendanceWorkPolicyResponse> {
	const saved = await supabase().rpc('attendance_work_policy_save', {
		target_policy: currentAttendanceWorkPolicy(revision)
	});
	if (saved.error) throw new Error(saved.error.message);
	return supabaseAttendanceWorkPolicy();
}

export function workPolicyResponse(settings: CompanySettings): AttendanceWorkPolicyResponse {
	const stored = settings.rules.attendanceWorkPolicy;
	const currentMonth = monthIn(settings.timeZone);
	const revision: AttendanceWorkPolicyRevision = {
		effectiveDate: initialWorkPolicyEffectiveDate,
		...(stored === undefined ? defaultWorkPolicy() : currentAttendanceWorkPolicy(stored))
	};
	return {
		policy: { version: 1, updatedAt: '', revisions: [revision] },
		currentMonth,
		holidayDates: companyHolidayDatesInMonth(settings.rules.companyHolidays ?? [], currentMonth),
		timeZone: settings.timeZone
	};
}

export function monthIn(timeZone: string): string {
	const parts = new Intl.DateTimeFormat('en-US', {
		timeZone,
		year: 'numeric',
		month: '2-digit'
	}).formatToParts(new Date());
	const year = parts.find((part) => part.type === 'year')?.value ?? '';
	const month = parts.find((part) => part.type === 'month')?.value ?? '';
	return `${year}-${month}`;
}
