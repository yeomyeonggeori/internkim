import { invokeTool } from '$lib/public-api-call';
import { companyHolidayDatesInMonth } from './company-holiday-dates';
import { currentAttendanceWorkPolicy } from './current-work-policy';
import { supabaseCompanyHolidays } from './supabase-company-holidays';
import { defaultWorkPolicy, initialWorkPolicyEffectiveDate } from './work-policy-defaults';
import type {
	AttendanceWorkPolicyResponse,
	AttendanceWorkPolicyRevision,
	CompanyHoliday
} from '../../routes/admin/admin-types';

export type AnsweredWorkPolicy = {
	timeZone: string;
	workMode: string;
	policy: { version: number; revisions: AttendanceWorkPolicyRevision[] } | null;
	people: { personID: string; workHours: unknown[][] | null; minimumDailyMinutes: number | null }[];
};

export function answeredWorkPolicy(): Promise<AnsweredWorkPolicy> {
	return invokeTool<AnsweredWorkPolicy>('attendance_work_policy_get', {});
}

export async function supabaseAttendanceWorkPolicy(): Promise<AttendanceWorkPolicyResponse> {
	return workPolicyResponse(await answeredWorkPolicy(), await supabaseCompanyHolidays());
}

export async function saveSupabaseAttendanceWorkPolicy(
	revision: AttendanceWorkPolicyRevision
): Promise<AttendanceWorkPolicyResponse> {
	await invokeTool('attendance_work_policy_set', {
		...currentAttendanceWorkPolicy(revision)
	});
	return supabaseAttendanceWorkPolicy();
}

export function workPolicyResponse(
	answered: AnsweredWorkPolicy,
	holidays: CompanyHoliday[]
): AttendanceWorkPolicyResponse {
	const currentMonth = monthIn(answered.timeZone);
	const revisions = answered.policy?.revisions ?? [
		{ effectiveDate: initialWorkPolicyEffectiveDate, ...defaultWorkPolicy() }
	];
	return {
		policy: { version: 1, updatedAt: '', revisions },
		currentMonth,
		holidayDates: companyHolidayDatesInMonth(holidays, currentMonth),
		timeZone: answered.timeZone
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
