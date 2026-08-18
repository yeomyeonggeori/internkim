import type {
	LeaveManagementAdjustment,
	LeaveManagementLegacyMigrationBatch,
	LeaveManagementLegacyMigrationPreview,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from './leave-management-types';
import { EmployeeLeaveAPIError } from '../leave/employee-leave-api';
import { isEmployeeLeaveErrorCode } from '../leave/employee-leave-types';
import {
	adjustSupabaseManagedLeave,
	cancelSupabaseManagedLeave,
	correctSupabaseManagedLeaveTime,
	createSupabaseManagedPastLeave,
	supabaseLeaveManagement
} from '$lib/attendance/supabase-leave-management';
import { isSupabaseConfigured } from '$lib/supabase';

export async function fetchLeaveManagement(
	employeeEmail = ''
): Promise<LeaveManagementPayload> {
	if (isSupabaseConfigured()) return supabaseLeaveManagement(employeeEmail);
	const query = new URLSearchParams();
	if (employeeEmail) query.set('email', employeeEmail);
	const queryString = query.toString();
	const response = await fetch(
		queryString
			? `/attendance/api/leave-management?${queryString}`
			: '/attendance/api/leave-management',
		{ credentials: 'include', cache: 'no-store' }
	);
	return readJSON<LeaveManagementPayload>(response);
}

export async function fetchLegacyAbsenceMigrationPreview(): Promise<
	LeaveManagementLegacyMigrationPreview
> {
	const response = await fetch('/attendance/api/leave-management/legacy-migration', {
		credentials: 'include',
		cache: 'no-store'
	});
	return readJSON<LeaveManagementLegacyMigrationPreview>(response);
}

export async function applyLegacyAbsenceMigration(
	fingerprint: string
): Promise<LeaveManagementLegacyMigrationBatch> {
	const response = await fetch('/attendance/api/leave-management/legacy-migration/apply', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ fingerprint })
	});
	const payload = await readJSON<{ batch: LeaveManagementLegacyMigrationBatch }>(response);
	return payload.batch;
}

export async function adjustManagedLeave(
	input: LeaveManagementAdjustment
): Promise<void> {
	if (isSupabaseConfigured()) return adjustSupabaseManagedLeave(input);
	const response = await fetch('/attendance/api/leave-management/adjustments', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	await readJSON(response);
}

export async function createManagedPastLeave(
	input: LeaveManagementPastLeave
): Promise<void> {
	if (isSupabaseConfigured()) return createSupabaseManagedPastLeave(input);
	const response = await fetch('/attendance/api/leave-management/past-leaves', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	await readJSON(response);
}

export async function cancelManagedLeaveRequest(
	requestID: string,
	employeeEmail: string
): Promise<void> {
	if (isSupabaseConfigured()) return cancelSupabaseManagedLeave(requestID, employeeEmail);
	const response = await fetch(
		`/attendance/api/leave-management/requests/${encodeURIComponent(requestID)}/cancel`,
		{
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ employeeEmail })
		}
	);
	await readJSON(response);
}

export async function correctManagedLeaveTime(
	requestID: string,
	input: LeaveManagementTimeCorrection
): Promise<void> {
	if (isSupabaseConfigured()) return correctSupabaseManagedLeaveTime(requestID, input);
	const response = await fetch(
		`/attendance/api/leave-management/requests/${encodeURIComponent(requestID)}/time`,
		{
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(input)
		}
	);
	await readJSON(response);
}

async function readJSON<Value = unknown>(response: Response): Promise<Value> {
	if (!response.ok) {
		const payload: unknown = await response.json().catch(() => null);
		const candidateCode =
			payload && typeof payload === 'object' && !Array.isArray(payload)
				? (payload as Record<string, unknown>).code
				: null;
		throw new EmployeeLeaveAPIError(
			isEmployeeLeaveErrorCode(candidateCode) ? candidateCode : null,
			response.status
		);
	}
	return (await response.json()) as Value;
}
