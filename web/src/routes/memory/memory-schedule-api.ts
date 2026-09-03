import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import type { MemoryChange } from './memory-change';

export type MemorySchedule = {
	taskScheduleID: string;
	creatorPersonID?: string;
	name?: string;
	executionMode: string;
	kind: string;
	intervalSecond?: number;
	cronExpression?: string;
	maxRunCount?: number;
	completedRunCount?: number;
	createdAt?: string;
	updatedAt?: string;
	nextRunAt?: string;
	lastRunAt?: string;
	expiresAt?: string;
	failureCount?: number;
	deliveryChannelID?: string;
	replyTargetID?: string;
	promptPreview?: string;
	timeZone?: string;
};

export type MemoryScheduleListResponse = {
	schedules?: MemorySchedule[];
	count?: number;
	totalCount?: number;
	page?: number;
	pageSize?: number;
	checkedAt?: string;
};

export type MemoryScheduleListRequest = {
	page?: number;
	pageSize?: number;
	includeExpired?: boolean;
};

export type ScheduleUpdateFields = {
	name?: string;
	kind?: 'once' | 'interval' | 'cron';
	runAt?: string;
	intervalSecond?: number;
	cronExpression?: string;
	timeZone?: string;
	expiresAt?: string;
	maxRunCount?: number;
	repeatPolicy?: 'finite' | 'unbounded';
};

export async function fetchMemorySchedules(request: MemoryScheduleListRequest = {}): Promise<MemoryScheduleListResponse> {
	return normalizeMemoryScheduleListResponse(
		isSupabaseConfigured() ? await askTheCompanyApp(request) : await askTheDevice(request)
	);
}

export function memoryScheduleListBody(request: MemoryScheduleListRequest): Record<string, unknown> {
	const body: Record<string, unknown> = {};
	if (typeof request.page === 'number' && request.page > 0) body.page = Math.floor(request.page);
	if (typeof request.pageSize === 'number' && request.pageSize > 0) body.pageSize = Math.floor(request.pageSize);
	if (typeof request.includeExpired === 'boolean') body.includeExpired = request.includeExpired;
	return body;
}

export function scheduleCancelRequest(taskScheduleID: string): MemoryChange {
	return {
		capability: 'person.memory.schedule_cancel',
		path: '/memory/api/schedules/cancel',
		body: { taskScheduleID }
	};
}

export function scheduleDeleteRequest(taskScheduleID: string): MemoryChange {
	return {
		capability: 'person.memory.schedule_delete',
		path: '/memory/api/schedules/delete',
		body: { taskScheduleID }
	};
}

export function scheduleUpdateRequest(taskScheduleID: string, fields: ScheduleUpdateFields): MemoryChange {
	return {
		capability: 'person.memory.schedule_update',
		path: '/memory/api/schedules/update',
		body: { taskScheduleID, ...fields }
	};
}

export async function cancelSchedule(taskScheduleID: string): Promise<void> {
	await changeSchedule(scheduleCancelRequest(taskScheduleID));
}

export async function deleteSchedule(taskScheduleID: string): Promise<void> {
	await changeSchedule(scheduleDeleteRequest(taskScheduleID));
}

export async function updateSchedule(taskScheduleID: string, fields: ScheduleUpdateFields): Promise<void> {
	await changeSchedule(scheduleUpdateRequest(taskScheduleID, fields));
}

async function askTheDevice(request: MemoryScheduleListRequest): Promise<unknown> {
	const response = await adminApiFetch(memorySchedulesURL(request));
	if (!response.ok) {
		throw new Error(`Memory schedules request returned ${response.status}`);
	}
	return response.json();
}

async function askTheCompanyApp(request: MemoryScheduleListRequest): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.memory.schedules', body: memoryScheduleListBody(request) });
	if (answer.status >= 400) throw new Error(`Memory schedules request returned ${answer.status}`);
	return answer.body;
}

export function normalizeMemoryScheduleListResponse(document: unknown): MemoryScheduleListResponse {
	const record = readRecord(document);
	if (!record) return {};

	const schedules = Array.isArray(record.schedules)
		? record.schedules.flatMap((schedule) => {
				const normalizedSchedule = normalizeMemorySchedule(schedule);
				return normalizedSchedule ? [normalizedSchedule] : [];
			})
		: undefined;
	const count = readNumber(record.count);
	const totalCount = readNumber(record.totalCount);
	const page = readNumber(record.page);
	const pageSize = readNumber(record.pageSize);
	const checkedAt = readString(record.checkedAt);

	return {
		...(schedules ? { schedules } : {}),
		...(typeof count === 'number' ? { count } : {}),
		...(typeof totalCount === 'number' ? { totalCount } : {}),
		...(typeof page === 'number' ? { page } : {}),
		...(typeof pageSize === 'number' ? { pageSize } : {}),
		...(checkedAt ? { checkedAt } : {})
	};
}

async function changeSchedule(change: MemoryChange): Promise<void> {
	if (isSupabaseConfigured()) {
		const answer = await callCompanyApp({ capability: change.capability, body: change.body });
		if (answer.status >= 400) throw new Error(`Memory schedules request returned ${answer.status}`);
		return;
	}
	const response = await adminApiFetch(change.path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(change.body)
	});
	if (!response.ok) {
		throw new Error(`Memory schedules request returned ${response.status}`);
	}
}

function memorySchedulesURL(request: MemoryScheduleListRequest): string {
	const query = new URLSearchParams();
	setPositiveIntegerQuery(query, 'page', request.page);
	setPositiveIntegerQuery(query, 'pageSize', request.pageSize);
	setBooleanQuery(query, 'includeExpired', request.includeExpired);
	const queryString = query.toString();
	return queryString ? `/memory/api/schedules?${queryString}` : '/memory/api/schedules';
}

function setPositiveIntegerQuery(query: URLSearchParams, name: string, value: number | undefined): void {
	if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return;
	query.set(name, String(Math.floor(value)));
}

function setBooleanQuery(query: URLSearchParams, name: string, value: boolean | undefined): void {
	if (typeof value !== 'boolean') return;
	query.set(name, value ? 'true' : 'false');
}

function normalizeMemorySchedule(document: unknown): MemorySchedule | undefined {
	const record = readRecord(document);
	if (!record) return undefined;

	const taskScheduleID = readString(record.taskScheduleID);
	const executionMode = readString(record.executionMode);
	const kind = readString(record.kind);
	if (!taskScheduleID || !executionMode || !kind) return undefined;

	const creatorPersonID = readString(record.creatorPersonID);
	const name = readString(record.name);
	const intervalSecond = readNumber(record.intervalSecond);
	const cronExpression = readString(record.cronExpression);
	const maxRunCount = readNumber(record.maxRunCount);
	const completedRunCount = readNumber(record.completedRunCount);
	const createdAt = readString(record.createdAt);
	const updatedAt = readString(record.updatedAt);
	const nextRunAt = readString(record.nextRunAt);
	const lastRunAt = readString(record.lastRunAt);
	const expiresAt = readString(record.expiresAt);
	const failureCount = readNumber(record.failureCount);
	const deliveryChannelID = readString(record.deliveryChannelID);
	const replyTargetID = readString(record.replyTargetID);
	const promptPreview = readString(record.promptPreview);
	const timeZone = readString(record.timeZone);

	return {
		taskScheduleID,
		executionMode,
		kind,
		...(creatorPersonID ? { creatorPersonID } : {}),
		...(name ? { name } : {}),
		...(typeof intervalSecond === 'number' ? { intervalSecond } : {}),
		...(cronExpression ? { cronExpression } : {}),
		...(typeof maxRunCount === 'number' ? { maxRunCount } : {}),
		...(typeof completedRunCount === 'number' ? { completedRunCount } : {}),
		...(createdAt ? { createdAt } : {}),
		...(updatedAt ? { updatedAt } : {}),
		...(nextRunAt ? { nextRunAt } : {}),
		...(lastRunAt ? { lastRunAt } : {}),
		...(expiresAt ? { expiresAt } : {}),
		...(typeof failureCount === 'number' ? { failureCount } : {}),
		...(deliveryChannelID ? { deliveryChannelID } : {}),
		...(replyTargetID ? { replyTargetID } : {}),
		...(promptPreview ? { promptPreview } : {}),
		...(timeZone ? { timeZone } : {})
	};
}

function readRecord(document: unknown): Record<string, unknown> | undefined {
	return isRecord(document) ? document : undefined;
}

function isRecord(document: unknown): document is Record<string, unknown> {
	return Boolean(document) && typeof document === 'object' && !Array.isArray(document);
}

function readString(value: unknown): string | undefined {
	return typeof value === 'string' ? value : undefined;
}

function readNumber(value: unknown): number | undefined {
	return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}
