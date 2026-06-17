import type { MemorySchedule, ScheduleUpdateFields } from './memory-schedule-api';
import type { MemoryText } from './text';

export type ScheduleKind = 'once' | 'interval' | 'cron';
export type RepeatPolicy = 'finite' | 'unbounded';

export type ScheduleEditDraft = {
	taskScheduleID: string;
	name: string;
	kind: ScheduleKind;
	runAt: string;
	intervalMinute: string;
	cronExpression: string;
	timeZone: string;
	expiresAt: string;
	maxRunCount: string;
	repeatPolicy: RepeatPolicy;
};

export const emptyScheduleEditDraft: ScheduleEditDraft = {
	taskScheduleID: '',
	name: '',
	kind: 'once',
	runAt: '',
	intervalMinute: '60',
	cronExpression: '',
	timeZone: 'Asia/Seoul',
	expiresAt: '',
	maxRunCount: '',
	repeatPolicy: 'unbounded'
};

export function createScheduleEditDraft(schedule: MemorySchedule): ScheduleEditDraft {
	return {
		taskScheduleID: schedule.taskScheduleID,
		name: schedule.name ?? '',
		kind: normalizedScheduleKind(schedule.kind),
		runAt: dateTimeInputValue(schedule.nextRunAt),
		intervalMinute: schedule.intervalSecond ? String(Math.max(1, Math.round(schedule.intervalSecond / 60))) : '60',
		cronExpression: schedule.cronExpression ?? '',
		timeZone: schedule.timeZone ?? 'Asia/Seoul',
		expiresAt: dateTimeInputValue(schedule.expiresAt),
		maxRunCount: schedule.maxRunCount && schedule.maxRunCount > 0 ? String(schedule.maxRunCount) : '',
		repeatPolicy: schedule.maxRunCount || schedule.expiresAt ? 'finite' : 'unbounded'
	};
}

export function createScheduleUpdateFields(draft: ScheduleEditDraft): ScheduleUpdateFields | undefined {
	const fields: ScheduleUpdateFields = {
		kind: draft.kind
	};
	const name = draft.name.trim();
	if (name) fields.name = name;
	if (draft.kind === 'once') {
		const runAt = dateTimeInputToISOString(draft.runAt);
		if (!runAt) return undefined;
		fields.runAt = runAt;
		return fields;
	}
	if (draft.kind === 'interval') {
		const intervalMinute = positiveInteger(draft.intervalMinute);
		if (!intervalMinute) return undefined;
		fields.intervalSecond = intervalMinute * 60;
	}
	if (draft.kind === 'cron') {
		const cronExpression = draft.cronExpression.trim();
		if (!cronExpression) return undefined;
		fields.cronExpression = cronExpression;
		fields.timeZone = draft.timeZone.trim() || 'Asia/Seoul';
	}
	fields.repeatPolicy = draft.repeatPolicy;
	const expiresAt = dateTimeInputToISOString(draft.expiresAt);
	if (expiresAt) fields.expiresAt = expiresAt;
	const maxRunCount = positiveInteger(draft.maxRunCount);
	if (maxRunCount) fields.maxRunCount = maxRunCount;
	return fields;
}

export function canSaveScheduleDraft(draft: ScheduleEditDraft, isSavingSchedule: boolean): boolean {
	return Boolean(createScheduleUpdateFields(draft)) && !isSavingSchedule;
}

export function scheduleKindLabel(text: MemoryText, kind: ScheduleKind): string {
	if (kind === 'cron') return text.scheduleKindCron;
	if (kind === 'interval') return text.scheduleKindInterval;
	return text.scheduleKindOnce;
}

export function normalizedScheduleKind(kind: string): ScheduleKind {
	if (kind === 'cron') return 'cron';
	if (kind === 'interval') return 'interval';
	return 'once';
}

function positiveInteger(value: string): number | undefined {
	const number = Number(value);
	if (!Number.isFinite(number) || number <= 0) return undefined;
	return Math.floor(number);
}

function dateTimeInputValue(value: string | undefined): string {
	if (!value) return '';
	const date = new Date(value);
	if (!Number.isFinite(date.getTime())) return '';
	const year = date.getFullYear();
	const month = paddedDatePart(date.getMonth() + 1);
	const day = paddedDatePart(date.getDate());
	const hour = paddedDatePart(date.getHours());
	const minute = paddedDatePart(date.getMinutes());
	return `${year}-${month}-${day}T${hour}:${minute}`;
}

function dateTimeInputToISOString(value: string): string | undefined {
	if (!value.trim()) return undefined;
	const date = new Date(value);
	if (!Number.isFinite(date.getTime())) return undefined;
	return date.toISOString();
}

function paddedDatePart(value: number): string {
	return String(value).padStart(2, '0');
}
