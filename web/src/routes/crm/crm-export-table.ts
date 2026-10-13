import type { Money } from '$lib/money';
import { taskStatusLabelFrom } from '../task/task-status';
import { crmDefinitionLabel, crmLabel } from './crm-labels';
import type {
	CRMActivity,
	CRMContact,
	CRMOpportunity,
	CRMOrganization,
	CRMPipeline,
	CRMPipelineStage
} from './crm-types';
import { findOrganizationByID, opportunityStageLabel } from './crm-view-model';
import type { CRMText } from './text';

export const crmExportTabs = ['relationships', 'contacts', 'pipeline', 'activities'] as const;
export type CRMExportTab = (typeof crmExportTabs)[number];

export type CRMExportRow = {
	id: string;
	organizationID: string;
	cells: string[];
};

export type CRMExportTable = {
	columns: string[];
	rows: CRMExportRow[];
};

export type CRMExportSource = {
	organizations: readonly CRMOrganization[];
	contacts: readonly CRMContact[];
	opportunities: readonly CRMOpportunity[];
	activities: readonly CRMActivity[];
	organizationTypeDefinitions: readonly { id: string; name: string }[];
	pipelines: readonly CRMPipeline[];
	stages: readonly CRMPipelineStage[];
	companyBaseCurrency: string;
	timeZone: string;
	text: CRMText;
};

const listSeparator = ', ';

export function crmExportTable(tab: CRMExportTab, source: CRMExportSource): CRMExportTable {
	if (tab === 'relationships') return organizationTable(source);
	if (tab === 'contacts') return contactTable(source);
	if (tab === 'pipeline') return opportunityTable(source);
	return activityTable(source);
}

export function crmExportFilename(tabLabel: string, date: string): string {
	return `CRM-${tabLabel}-${date}.csv`;
}

export function crmExportRowMatches(row: CRMExportRow, query: string): boolean {
	const needle = query.trim().toLocaleLowerCase();
	if (!needle) return true;
	return row.cells.some((cell) => cell.toLocaleLowerCase().includes(needle));
}

function organizationTable(source: CRMExportSource): CRMExportTable {
	const { text } = source;
	return {
		columns: [text.name, text.type, text.status, text.importance, text.internalOwner, text.ownerEmail, text.address, text.tags, text.details],
		rows: source.organizations.map((organization) => ({
			id: organization.id,
			organizationID: organization.id,
			cells: [
				organization.name,
				organization.types
					.map((type) => crmDefinitionLabel(source.organizationTypeDefinitions, text.organizationTypes, type))
					.join(listSeparator),
				crmLabel(text.organizationStatuses, organization.status),
				crmLabel(text.importanceLabels, organization.importance),
				organization.ownerName,
				organization.ownerEmail,
				organization.address ?? '',
				organization.tags.join(listSeparator),
				organization.description
			]
		}))
	};
}

function contactTable(source: CRMExportSource): CRMExportTable {
	const { text } = source;
	return {
		columns: [text.contactName, text.organizationName, text.contactTitle, text.export.department, text.email, text.phone, text.details],
		rows: source.contacts.map((contact) => ({
			id: contact.id,
			organizationID: contact.organizationID,
			cells: [
				contact.name,
				organizationName(source, contact.organizationID),
				contact.title,
				contact.department ?? '',
				contact.email,
				contact.phone ?? '',
				contact.note ?? ''
			]
		}))
	};
}

function opportunityTable(source: CRMExportSource): CRMExportTable {
	const { text } = source;
	return {
		columns: [
			text.name,
			text.organizationName,
			text.externalContact,
			text.business,
			text.progressKind,
			text.stage,
			text.amount,
			text.currency,
			text.export.baseAmount.replace('{currency}', source.companyBaseCurrency),
			text.targetDate,
			text.internalOwner,
			text.importance,
			text.lostReasonPrompt,
			text.details
		],
		rows: source.opportunities.map((opportunity) => ({
			id: opportunity.id,
			organizationID: opportunity.organizationID,
			cells: [
				opportunity.name,
				organizationName(source, opportunity.organizationID),
				(opportunity.contacts ?? []).map((link) => contactName(source, link.contactID)).join(listSeparator),
				opportunity.business,
				pipelineLabel(source, opportunity.pipeline ?? opportunity.kind ?? ''),
				opportunityStageLabel(source.stages, opportunity.stage, text),
				amountText(opportunity.amount),
				opportunity.amount?.currency ?? '',
				amountText(opportunity.settledAmount),
				opportunity.targetDate,
				opportunity.ownerName,
				crmLabel(text.importanceLabels, opportunity.importance),
				opportunity.lostReason ?? '',
				opportunity.description ?? ''
			]
		}))
	};
}

function activityTable(source: CRMExportSource): CRMExportTable {
	const { text } = source;
	return {
		columns: [
			text.activityTitle,
			text.organizationName,
			text.relatedProgress,
			text.activityKind,
			text.occurredAt,
			text.business,
			text.externalContact,
			text.assignee,
			text.activityStatus,
			text.columnSize,
			text.registerCalendar,
			text.startTime,
			text.endTime,
			text.location,
			text.details
		],
		rows: source.activities.map((activity) => ({
			id: activity.id,
			organizationID: activity.organizationID,
			cells: [
				activity.title,
				organizationName(source, activity.organizationID),
				opportunityName(source, activity.opportunityID),
				crmLabel(text.activityKinds, activity.kind),
				localDateTime(activity.occurredAt, source.timeZone),
				activity.business,
				activity.contactID ? contactName(source, activity.contactID) : '',
				activity.participantNames.join(listSeparator),
				taskStatusLabelFrom(text.taskStatuses, activity.taskStatus),
				activity.size,
				activity.calendarEventID ? text.calendarRegistered : '',
				calendarTime(activity.calendarEventDate, activity, source.timeZone),
				calendarTime(activity.calendarEndsAt, activity, source.timeZone),
				activity.calendarLocation ?? '',
				activity.summary
			]
		}))
	};
}

function organizationName(source: CRMExportSource, organizationID: string): string {
	return findOrganizationByID([...source.organizations], organizationID)?.name ?? '';
}

function contactName(source: CRMExportSource, contactID: string): string {
	return source.contacts.find((contact) => contact.id === contactID)?.name ?? '';
}

function opportunityName(source: CRMExportSource, opportunityID: string | undefined): string {
	if (!opportunityID) return '';
	return source.opportunities.find((opportunity) => opportunity.id === opportunityID)?.name ?? '';
}

function pipelineLabel(source: CRMExportSource, pipeline: string): string {
	if (!pipeline) return '';
	return source.pipelines.find((candidate) => candidate.pipeline === pipeline)?.label ?? crmLabel(source.text.progressKinds, pipeline);
}

function amountText(money: Money | undefined): string {
	return money === undefined ? '' : String(money.amount);
}

function calendarTime(dateTime: string | undefined, activity: CRMActivity, timeZone: string): string {
	if (!activity.calendarEventID || !dateTime) return '';
	const local = localDateTime(dateTime, timeZone);
	return activity.isWholeDay ? local.slice(0, 10) : local;
}

function localDateTime(dateTime: string, timeZone: string): string {
	const instant = new Date(dateTime);
	if (Number.isNaN(instant.getTime())) return dateTime;
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(instant);
	const part = (type: Intl.DateTimeFormatPartTypes): string => parts.find((candidate) => candidate.type === type)?.value ?? '';
	return `${part('year')}-${part('month')}-${part('day')} ${part('hour')}:${part('minute')}`;
}
