import type { Event as DayFlowEvent } from '@dayflow/core';

export type CalendarAuditRow = {
	label: string;
	person: string;
	time: string;
};

export type CalendarAuditLabels = {
	created: string;
	updated: string;
};

export function calendarAuditRows(
	event: Pick<DayFlowEvent, 'meta'> | null,
	localeCode: string,
	labels: CalendarAuditLabels
): CalendarAuditRow[] {
	if (!event) return [];
	const rows: CalendarAuditRow[] = [];
	const createdBy = auditPerson(event.meta?.createdByName, event.meta?.createdByEmail);
	if (createdBy) {
		rows.push({ label: labels.created, person: createdBy, time: '' });
	}
	const updatedBy = auditPerson(event.meta?.updatedByName, event.meta?.updatedByEmail);
	const updatedAt = auditTime(event.meta?.updatedByAt, localeCode);
	if (updatedBy && updatedAt) {
		rows.push({ label: labels.updated, person: updatedBy, time: updatedAt });
	}
	return rows;
}

function auditPerson(name: unknown, email: unknown): string {
	return stringMeta(name) || stringMeta(email);
}

function auditTime(value: unknown, localeCode: string): string {
	const date = new Date(stringMeta(value));
	if (Number.isNaN(date.getTime())) return '';
	return new Intl.DateTimeFormat(localeCode, {
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
