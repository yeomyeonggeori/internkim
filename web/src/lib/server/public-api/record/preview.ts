import type { z } from 'zod';
import { holidayOfCompanyHint } from './company-tools';
import { companyRecordOfHint } from './company-ledger-tools';
import { contactOfCRMHint, opportunityOfCRMHint, organizationOfCRMHint } from './crm-tools';
import { momentIn } from './days';
import { eventOfHint } from './event-tools';
import { personOfHint } from './people';
import { taskRowOfHint } from './task-tools';
import type { RecordContext } from './company';
import type { TaskRow } from './tasks';
import type { calendarDeleteInputSchema, taskDeleteInputSchema } from '../catalog/tools';
import type { companyHolidayDeleteInputSchema } from '../catalog/settings';
import type { companyRecordDeleteInputSchema } from '../catalog/company';
import type {
	crmContactArchiveInputSchema,
	crmOpportunityArchiveInputSchema,
	crmOrganizationArchiveInputSchema
} from '../catalog/crm';
import type { personWithdrawInputSchema } from '../catalog/people';

export type PreviewedTarget = {
	inputField: string;
	id?: string;
	ids?: string[];
	title: string;
	startsAt?: string;
	preview?: string;
};

export async function taskDeleteTarget(
	context: RecordContext,
	input: z.infer<typeof taskDeleteInputSchema>
): Promise<PreviewedTarget> {
	return targetOf('taskHint', await taskRowOfHint(context, input.taskHint), context);
}

export async function eventDeleteTarget(
	context: RecordContext,
	input: z.infer<typeof calendarDeleteInputSchema>
): Promise<PreviewedTarget> {
	return targetOf('eventHint', await eventOfHint(context, input.eventHint), context);
}

export async function companyHolidayDeleteTarget(
	context: RecordContext,
	input: z.infer<typeof companyHolidayDeleteInputSchema>
): Promise<PreviewedTarget> {
	const holiday = await holidayOfCompanyHint(context, input.holidayHint);
	return { inputField: 'holidayHint', id: holiday.id, title: `${holiday.date} ${holiday.title}` };
}

export async function companyRecordDeleteTarget(
	context: RecordContext,
	input: z.infer<typeof companyRecordDeleteInputSchema>
): Promise<PreviewedTarget> {
	const record = await companyRecordOfHint(context, input.recordHint);
	return { inputField: 'recordHint', id: record.id, title: record.title };
}

export async function crmOrganizationArchiveTarget(
	context: RecordContext,
	input: z.infer<typeof crmOrganizationArchiveInputSchema>
): Promise<PreviewedTarget> {
	const organization = await organizationOfCRMHint(context, input.organizationHint);
	return { inputField: 'organizationHint', id: organization.id, title: organization.name };
}

export async function crmContactArchiveTarget(
	context: RecordContext,
	input: z.infer<typeof crmContactArchiveInputSchema>
): Promise<PreviewedTarget> {
	const contact = await contactOfCRMHint(context, input.contactHint);
	return { inputField: 'contactHint', id: contact.id, title: contact.name };
}

export async function crmOpportunityArchiveTarget(
	context: RecordContext,
	input: z.infer<typeof crmOpportunityArchiveInputSchema>
): Promise<PreviewedTarget> {
	const opportunity = await opportunityOfCRMHint(context, input.opportunityHint);
	return { inputField: 'opportunityHint', id: opportunity.id, title: opportunity.name };
}

export async function personTarget(
	context: RecordContext,
	input: z.infer<typeof personWithdrawInputSchema>
): Promise<PreviewedTarget> {
	const person = personOfHint(context.people, input.personHint);
	return { inputField: 'personHint', id: person.personID, title: `${person.name} <${person.email}>` };
}

function targetOf(inputField: string, row: TaskRow, context: RecordContext): PreviewedTarget {
	return {
		inputField,
		id: row.id,
		title: row.title,
		...(row.starts_at ? { startsAt: momentIn(context.labels.timezone, row.starts_at) } : {})
	};
}
