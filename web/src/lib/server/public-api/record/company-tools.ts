import type { SupabaseClient } from '@supabase/supabase-js';
import { currentAttendanceWorkPolicy } from '$lib/attendance/current-work-policy';
import { defaultLeavePolicy, systemLeaveTypeKind } from '$lib/attendance/leave-policy-defaults';
import { defaultWorkPolicy, initialWorkPolicyEffectiveDate } from '$lib/attendance/work-policy-defaults';
import { storedWorkPolicyRevisions } from '$lib/attendance/stored-work-policy';
import { assetBucket } from '../asset-address';
import {
	companyProfileView,
	languageAsked,
	profileOf,
	profileWithUpdate,
	type CompanyProfileUpdate
} from './company-profile';
import { titleNearness } from './hint-nearness';
import { HintRefused, normalized, resolveHint, type HintMatcher } from './hint-resolution';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import type { RecordContext } from './company';
import { keepUploadedImages } from './service-files';
import type { ServiceFileImageField } from '$lib/data-room/service-files';
import type { CompanyProfileResult } from '../catalog/company';
import type {
	AttendanceLeavePolicyResult,
	AttendanceWorkPolicyResult,
	AttendanceWorkPolicySetResult,
	CompanyHolidayListResult,
	CompanyHolidayResult,
	CompanySettingsResult
} from '../catalog/settings';

export type CompanySettingsUpdateInput = {
	name?: string;
	locale?: string;
	timeZone?: string;
	currencyCode?: string;
	workLocations?: { name?: string; color?: string }[];
	leaveDays?: number;
	teamViewVisibleToAll?: boolean;
};

export type CompanyInfoGetInput = { language?: string };

export type CompanyInfoSetInput = CompanyProfileUpdate & Partial<Record<ServiceFileImageField, string>>;

export type CompanyHolidayListInput = { year?: number };

export type CompanyHolidayAddInput = { date?: string; name?: string; recursAnnually?: boolean };

export type CompanyHolidayUpdateInput = {
	holidayHint?: string;
	date?: string;
	name?: string;
	recursAnnually?: boolean;
};

export type CompanyHolidayDeleteInput = { holidayHint?: string };

export type AttendanceLeavePolicySetInput = {
	balanceTrackingMode?: string;
	fiscalYearStartMonth?: number;
	fiscalYearStartDay?: number;
	leaveTypes?: { id?: string; systemKind?: string; name?: string; isSystem?: boolean }[];
};

const readableForOneHour = 60 * 60;
const leaveDaysPerMilliDay = 1000;

type StoredWorkLocation = { name: string; color?: string };

type StoredHoliday = {
	id: string;
	title: string;
	date: string;
	recursAnnually: boolean;
	createdAt?: string;
	updatedAt?: string;
};

type CompanyRow = {
	id: string;
	name: string;
	country: string;
	locale: string;
	timezone: string;
	currency_code: string;
	work_locations: StoredWorkLocation[] | null;
	rules: Record<string, unknown> | null;
	profile: unknown;
	profile_image: string | null;
};

const companyColumns =
	'id, name, country, locale, timezone, currency_code, work_locations, rules, profile, profile_image';

async function companyRow(caller: SupabaseClient): Promise<CompanyRow> {
	const { data, error } = await caller
		.from('company')
		.select(companyColumns)
		.limit(1)
		.single<CompanyRow>();
	if (error) throw new Error(error.message);
	return data;
}

async function writeTheCompany(
	context: RecordContext,
	change: Record<string, unknown>,
	refusal: string
): Promise<void> {
	const row = await companyRow(context.caller);
	const { data, error } = await context.caller
		.from('company')
		.update(change)
		.eq('id', row.id)
		.select('id');
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	if ((data ?? []).length === 0) throw new RecordRefusedTheWrite(refusal, 403);
}

async function readableURLOf(caller: SupabaseClient, path: string | null): Promise<string | null> {
	if (!path) return null;
	const { data, error } = await caller.storage.from(assetBucket).createSignedUrl(path, readableForOneHour);
	if (error) throw new Error(error.message);
	return data?.signedUrl ?? null;
}

function teamViewVisibleTo(rules: Record<string, unknown> | null): boolean {
	return rules?.teamViewVisibleToAll !== false;
}

export async function companySettingsGet(context: RecordContext): Promise<CompanySettingsResult> {
	const row = await companyRow(context.caller);
	return {
		name: row.name,
		country: row.country,
		locale: row.locale,
		timeZone: row.timezone,
		currencyCode: row.currency_code,
		workLocations: (row.work_locations ?? []).map((location) => ({
			name: location.name,
			color: location.color ?? null
		})),
		leaveDays: annualGrantDaysOf(row.rules?.attendanceLeavePolicy),
		teamViewVisibleToAll: teamViewVisibleTo(row.rules),
		profileImageURL: await readableURLOf(context.caller, row.profile_image)
	};
}

export async function companySettingsUpdate(
	context: RecordContext,
	input: CompanySettingsUpdateInput
): Promise<CompanySettingsResult> {
	const change: Record<string, unknown> = {};
	if (input.name !== undefined) change.name = input.name.trim();
	if (input.locale !== undefined) change.locale = input.locale.trim();
	if (input.timeZone !== undefined) change.timezone = input.timeZone.trim();
	if (input.currencyCode !== undefined) change.currency_code = input.currencyCode.trim().toUpperCase();
	if (input.workLocations !== undefined) {
		change.work_locations = input.workLocations.map((location) => ({
			name: location.name?.trim() ?? '',
			...(location.color?.trim() ? { color: location.color.trim() } : {})
		}));
	}
	if (input.teamViewVisibleToAll !== undefined) {
		const held = await companyRow(context.caller);
		change.rules = { ...(held.rules ?? {}), teamViewVisibleToAll: input.teamViewVisibleToAll };
	}
	if (input.leaveDays === undefined && Object.keys(change).length === 0) {
		throw new Error('a settings change names at least one setting to change');
	}

	if (Object.keys(change).length > 0) {
		await writeTheCompany(context, change, 'only an administrator can change the company settings');
	}
	if (input.leaveDays !== undefined) await writeTheAnnualGrant(context, input.leaveDays);
	return companySettingsGet(context);
}

async function writeTheAnnualGrant(context: RecordContext, leaveDays: number): Promise<void> {
	const policy = await attendanceLeavePolicyGet(context);
	await attendanceLeavePolicySet(context, {
		...policy,
		balanceTrackingMode: 'managed',
		leaveTypes: policy.leaveTypes.map((leaveType) =>
			leaveType.id === 'annual'
				? { ...leaveType, grantAmountMilliDays: Math.round(leaveDays * leaveDaysPerMilliDay) }
				: leaveType
		)
	});
}

export async function companyInfoGet(
	context: RecordContext,
	input: CompanyInfoGetInput
): Promise<CompanyProfileResult> {
	const row = await companyRow(context.caller);
	return companyProfileView(profileOf(row.profile), languageAsked(input.language));
}

export async function companyInfoSet(
	context: RecordContext,
	input: CompanyInfoSetInput
): Promise<CompanyProfileResult> {
	const language = languageAsked(input.language);
	const row = await companyRow(context.caller);
	const written = profileWithUpdate(profileOf(row.profile), input, language, context.now);

	await keepUploadedImages(context, input);
	await writeTheCompany(
		context,
		{ profile: written },
		'only an administrator can change the company master profile'
	);
	return companyProfileView(written, language);
}

function holidaysHeld(rules: Record<string, unknown> | null): StoredHoliday[] {
	const held = rules?.companyHolidays;
	if (!Array.isArray(held)) return [];
	return held.filter((holiday): holiday is StoredHoliday => isStoredHoliday(holiday));
}

function isStoredHoliday(value: unknown): boolean {
	if (typeof value !== 'object' || value === null) return false;
	const holiday = value as Record<string, unknown>;
	return typeof holiday.id === 'string' && typeof holiday.title === 'string' && typeof holiday.date === 'string';
}

function answeredHoliday(holiday: StoredHoliday): CompanyHolidayResult {
	return {
		holidayID: holiday.id,
		name: holiday.title,
		date: holiday.date,
		recursAnnually: holiday.recursAnnually === true,
		createdAt: holiday.createdAt ?? null,
		updatedAt: holiday.updatedAt ?? null
	};
}

function inReadingOrder(holidays: StoredHoliday[]): StoredHoliday[] {
	return [...holidays].sort(
		(left, right) => left.date.localeCompare(right.date) || left.title.localeCompare(right.title)
	);
}

async function saveHolidays(context: RecordContext, holidays: StoredHoliday[]): Promise<void> {
	const { error } = await context.caller.rpc('company_holidays_save', { target_holidays: holidays });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
}

const holidayMatcher: HintMatcher<StoredHoliday> = {
	identifiersOf: (holiday) => [holiday.id, holiday.date],
	titleOf: (holiday) => holiday.title,
	nearnessTo: (holiday, hint) => titleNearness(normalized(hint), normalized(holiday.title))
};

function hintOf(offered: string | undefined): string {
	const hint = offered?.trim();
	if (!hint) throw new Error('this call names the holiday it is about');
	return hint;
}

function holidayOfHint(holidays: StoredHoliday[], hint: string): StoredHoliday {
	const resolution = resolveHint(hint, holidays, holidayMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		'holiday',
		hint.trim(),
		resolution.outcome,
		resolution.candidates.map((holiday) => ({
			id: holiday.id,
			label: `${holiday.date} ${holiday.title}`
		}))
	);
}

export async function holidayOfCompanyHint(
	context: RecordContext,
	hint: string
): Promise<StoredHoliday> {
	const row = await companyRow(context.caller);
	return holidayOfHint(holidaysHeld(row.rules), hint);
}

export async function companyHolidayList(
	context: RecordContext,
	input: CompanyHolidayListInput
): Promise<CompanyHolidayListResult> {
	const row = await companyRow(context.caller);
	const held = inReadingOrder(holidaysHeld(row.rules));
	const kept =
		input.year === undefined
			? held
			: held.filter(
					(holiday) => holiday.recursAnnually === true || holiday.date.startsWith(`${input.year}-`)
				);
	return {
		count: kept.length,
		year: input.year ?? null,
		holidays: kept.map(answeredHoliday)
	};
}

export async function companyHolidayAdd(
	context: RecordContext,
	input: CompanyHolidayAddInput
): Promise<CompanyHolidayResult> {
	const row = await companyRow(context.caller);
	const held = holidaysHeld(row.rules);
	const date = input.date?.trim();
	const name = input.name?.trim();
	if (!date) throw new Error('a company holiday names the day it falls on');
	if (!name) throw new Error('a company holiday needs a name');
	refuseADayAlreadyHeld(held, date, '');

	const writtenAt = context.now.toISOString();
	const added: StoredHoliday = {
		id: `company-holiday-${crypto.randomUUID().replaceAll('-', '')}`,
		title: name,
		date,
		recursAnnually: input.recursAnnually === true,
		createdAt: writtenAt,
		updatedAt: writtenAt
	};
	await saveHolidays(context, [...held, added]);
	return answeredHoliday(added);
}

export async function companyHolidayUpdate(
	context: RecordContext,
	input: CompanyHolidayUpdateInput
): Promise<CompanyHolidayResult> {
	const row = await companyRow(context.caller);
	const held = holidaysHeld(row.rules);
	const holiday = holidayOfHint(held, hintOf(input.holidayHint));
	const date = input.date?.trim() || holiday.date;
	refuseADayAlreadyHeld(held, date, holiday.id);

	const written: StoredHoliday = {
		...holiday,
		title: input.name?.trim() || holiday.title,
		date,
		recursAnnually: input.recursAnnually ?? holiday.recursAnnually === true,
		updatedAt: context.now.toISOString()
	};
	await saveHolidays(
		context,
		held.map((candidate) => (candidate.id === holiday.id ? written : candidate))
	);
	return answeredHoliday(written);
}

export async function companyHolidayDelete(
	context: RecordContext,
	input: CompanyHolidayDeleteInput
): Promise<CompanyHolidayResult> {
	const row = await companyRow(context.caller);
	const held = holidaysHeld(row.rules);
	const holiday = holidayOfHint(held, hintOf(input.holidayHint));

	await saveHolidays(
		context,
		held.filter((candidate) => candidate.id !== holiday.id)
	);
	return answeredHoliday(holiday);
}

function refuseADayAlreadyHeld(held: StoredHoliday[], date: string, exceptID: string): void {
	const taken = held.find((holiday) => holiday.date === date && holiday.id !== exceptID);
	if (!taken) return;
	throw new RecordRefusedTheWrite(
		`this company already keeps ${taken.title} on ${date}`,
		409,
		'record_duplicate'
	);
}

type WorkPolicyRow = {
	member_id: string;
	work_hours: unknown;
	minimum_daily_minutes: number | null;
	work_mode: string;
	work_policy: unknown;
};

async function workPolicyRows(context: RecordContext): Promise<WorkPolicyRow[]> {
	const { data, error } = await context.caller.rpc('attendance_work_policies');
	if (error) throw new Error(error.message);
	if (!Array.isArray(data)) throw new Error('attendance work policies must be an array');
	return data as WorkPolicyRow[];
}

function workHoursOf(value: unknown, whose: string): unknown[][] | null {
	if (value === null || value === undefined) return null;
	if (!Array.isArray(value) || !value.every((week) => Array.isArray(week))) {
		throw new Error(`attendance work hours are invalid for ${whose}`);
	}
	return value as unknown[][];
}

function policyAnswered(stored: unknown): AttendanceWorkPolicyResult['policy'] {
	if (stored === null || stored === undefined) return null;
	return { version: 1, revisions: storedWorkPolicyRevisions(stored, 'this company') };
}

export async function attendanceWorkPolicyGet(
	context: RecordContext
): Promise<AttendanceWorkPolicyResult> {
	const rows = await workPolicyRows(context);
	const stored = rows.length > 0 ? rows[0].work_policy : null;
	const policy = policyAnswered(stored);
	return {
		timeZone: context.labels.timezone,
		workMode: policy
			? policy.revisions[policy.revisions.length - 1].workMode
			: rows[0]?.work_mode ?? defaultWorkPolicy().workMode,
		policy,
		people: rows.map((row) => ({
			personID: row.member_id,
			workHours: workHoursOf(row.work_hours, `member ${row.member_id}`),
			minimumDailyMinutes: row.minimum_daily_minutes
		}))
	};
}

export async function attendanceWorkPolicySet(
	context: RecordContext,
	input: Record<string, unknown>
): Promise<AttendanceWorkPolicySetResult> {
	const { error } = await context.caller.rpc('attendance_work_policy_save', {
		target_policy: currentAttendanceWorkPolicy(input)
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	const rows = await workPolicyRows(context);
	const policy = policyAnswered(rows.length > 0 ? rows[0].work_policy : null) ?? {
		version: 1,
		revisions: [{ ...currentAttendanceWorkPolicy(input), effectiveDate: initialWorkPolicyEffectiveDate }]
	};
	return {
		timeZone: context.labels.timezone,
		effectiveDate: policy.revisions[policy.revisions.length - 1].effectiveDate,
		policy
	};
}

export async function attendanceLeavePolicyGet(
	context: RecordContext
): Promise<AttendanceLeavePolicyResult> {
	const row = await companyRow(context.caller);
	return leavePolicyAnswered(row.rules?.attendanceLeavePolicy);
}

export async function attendanceLeavePolicySet(
	context: RecordContext,
	input: AttendanceLeavePolicySetInput
): Promise<AttendanceLeavePolicyResult> {
	for (const leaveType of input.leaveTypes ?? []) {
		const systemKind = systemLeaveTypeKind(leaveType.id ?? '');
		if (systemKind === undefined && (leaveType.isSystem || leaveType.systemKind)) {
			throw new Error(`leave type ${leaveType.id || leaveType.name} cannot claim a system identity`);
		}
		if (systemKind !== undefined && leaveType.isSystem && leaveType.systemKind !== systemKind) {
			throw new Error(`the system leave type ${leaveType.id} cannot change its kind`);
		}
	}

	const { error } = await context.caller.rpc('attendance_leave_policy_save', {
		target_policy: { ...input, version: 2 }
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return attendanceLeavePolicyGet(context);
}

function leavePolicyAnswered(stored: unknown): AttendanceLeavePolicyResult {
	if (isLeavePolicy(stored)) return stored;
	return unconfiguredLeavePolicy();
}

function unconfiguredLeavePolicy(): AttendanceLeavePolicyResult {
	return { ...defaultLeavePolicy(), balanceTrackingMode: 'unlimited' };
}

function annualGrantDaysOf(stored: unknown): number | null {
	const policy = leavePolicyAnswered(stored);
	if (policy.balanceTrackingMode === 'unlimited') return null;
	const annual = policy.leaveTypes.find((leaveType) => leaveType.id === 'annual');
	return annual ? annual.grantAmountMilliDays / leaveDaysPerMilliDay : null;
}

function isLeavePolicy(stored: unknown): stored is AttendanceLeavePolicyResult {
	if (typeof stored !== 'object' || stored === null) return false;
	const policy = stored as Record<string, unknown>;
	return Array.isArray(policy.leaveTypes) && typeof policy.fiscalYearStartMonth === 'number';
}
