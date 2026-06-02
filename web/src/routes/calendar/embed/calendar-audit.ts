import type { Event as DayFlowEvent } from '@dayflow/core';

export type CalendarAuditRow = {
	label: string;
	person: string;
	time: string;
};

export function calendarAuditRows(event: DayFlowEvent | null): CalendarAuditRow[] {
	if (!event) return [];
	const rows: CalendarAuditRow[] = [];
	const createdBy = auditPerson(event.meta?.createdByName, event.meta?.createdByEmail);
	if (createdBy) {
		rows.push({ label: '등록', person: createdBy, time: '' });
	}
	const updatedBy = auditPerson(event.meta?.updatedByName, event.meta?.updatedByEmail);
	const updatedAt = auditTime(event.meta?.updatedByAt);
	if (updatedBy && updatedAt) {
		rows.push({ label: '수정', person: updatedBy, time: updatedAt });
	}
	return rows;
}

function auditPerson(name: unknown, email: unknown): string {
	return stringMeta(name) || stringMeta(email);
}

function auditTime(value: unknown): string {
	const date = new Date(stringMeta(value));
	if (Number.isNaN(date.getTime())) return '';
	return new Intl.DateTimeFormat('ko-KR', {
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	}).format(date);
}

function stringMeta(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}
