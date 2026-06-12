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
	const response = await fetch(memorySchedulesURL(request), { credentials: 'include' });
	if (!response.ok) {
		throw new Error(`Memory schedules request returned ${response.status}`);
	}
	const document: unknown = await response.json();
	return normalizeMemoryScheduleListResponse(document);
}

export async function cancelSchedule(taskScheduleID: string): Promise<void> {
	await postMemoryScheduleRequest('/memory/api/schedules/cancel', { taskScheduleID });
}

export async function updateSchedule(taskScheduleID: string, fields: ScheduleUpdateFields): Promise<void> {
	await postMemoryScheduleRequest('/memory/api/schedules/update', { taskScheduleID, ...fields });
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

async function postMemoryScheduleRequest(path: string, body: Record<string, unknown>): Promise<void> {
	const response = await fetch(path, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (!response.ok) {
		throw new Error(`Memory schedules request returned ${response.status}`);
	}
}

function memorySchedulesURL(request: MemoryScheduleListRequest): string {
	const query = new URLSearchParams();
	setPositiveIntegerQuery(query, 'page', request.page);
	setPositiveIntegerQuery(query, 'pageSize', request.pageSize);
	const queryString = query.toString();
	return queryString ? `/memory/api/schedules?${queryString}` : '/memory/api/schedules';
}

function setPositiveIntegerQuery(query: URLSearchParams, name: string, value: number | undefined): void {
	if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return;
	query.set(name, String(Math.floor(value)));
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
