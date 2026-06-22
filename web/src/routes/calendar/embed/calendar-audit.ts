import type { Event as DayFlowEvent } from '@dayflow/core';

export type CalendarAuditRow = {
	label: string;
	actor: CalendarAuditActor;
	time: string;
};

export type CalendarAuditActor = {
	name: string;
	email: string;
	image: string;
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
	const createdBy = auditActor(event.meta?.createdByName, event.meta?.createdByEmail, event.meta?.createdByImage);
	if (createdBy) {
		rows.push({ label: labels.created, actor: createdBy, time: '' });
	}
	const updatedBy = auditActor(event.meta?.updatedByName, event.meta?.updatedByEmail, event.meta?.updatedByImage);
	const updatedAt = auditTime(event.meta?.updatedByAt, localeCode);
	if (updatedBy && updatedAt) {
		rows.push({ label: labels.updated, actor: updatedBy, time: updatedAt });
	}
	return rows;
}

function auditActor(name: unknown, email: unknown, image: unknown): CalendarAuditActor | null {
	const actorName = stringMeta(name) || stringMeta(email);
	const actorEmail = stringMeta(email);
	if (!actorName && !actorEmail) return null;
	return {
		name: actorName || actorEmail,
		email: actorEmail,
		image: stringMeta(image)
	};
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
