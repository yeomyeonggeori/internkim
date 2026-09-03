import { holidayOfCompanyHint } from './company-tools';
import { contactOfCRMHint, opportunityOfCRMHint, organizationOfCRMHint } from './crm-tools';
import { eventOfHint } from './event-tools';
import { taskRowOfHint } from './task-tools';
import type { RecordContext } from './company';
import type { TaskRow } from './tasks';

export type PreviewedTarget = {
	inputField: string;
	id: string;
	title: string;
	startsAt?: string;
};

type Preview = (context: RecordContext, input: Record<string, unknown>) => Promise<PreviewedTarget>;

// inputField is what blueclaw's narrowedToolInput replaces with the identity
// before it runs an approved call; without it the call replays the hint and
// resolves a second time.
const previewsOverTheRecord: Record<string, Preview> = {
	task_delete: async (context, input) =>
		targetOf('taskHint', await taskRowOfHint(context, hintOf(input, 'taskHint'))),
	event_delete: async (context, input) =>
		targetOf('eventHint', await eventOfHint(context, hintOf(input, 'eventHint'))),
	company_holiday_delete: async (context, input) => {
		const holiday = await holidayOfCompanyHint(context, hintOf(input, 'holidayHint'));
		return { inputField: 'holidayHint', id: holiday.id, title: `${holiday.date} ${holiday.title}` };
	},
	crm_organization_archive: async (context, input) => {
		const organization = await organizationOfCRMHint(context, hintOf(input, 'organizationHint'));
		return { inputField: 'organizationHint', id: organization.id, title: organization.name };
	},
	crm_contact_archive: async (context, input) => {
		const contact = await contactOfCRMHint(context, hintOf(input, 'contactHint'));
		return { inputField: 'contactHint', id: contact.id, title: contact.name };
	},
	crm_opportunity_archive: async (context, input) => {
		const opportunity = await opportunityOfCRMHint(context, hintOf(input, 'opportunityHint'));
		return { inputField: 'opportunityHint', id: opportunity.id, title: opportunity.name };
	}
};

export function previewOfTool(name: string): Preview | undefined {
	return previewsOverTheRecord[name];
}

function hintOf(input: Record<string, unknown>, field: string): string {
	const hint = input[field];
	return typeof hint === 'string' ? hint : '';
}

function targetOf(inputField: string, row: TaskRow): PreviewedTarget {
	return {
		inputField,
		id: row.id,
		title: row.title,
		...(row.starts_at ? { startsAt: row.starts_at } : {})
	};
}
