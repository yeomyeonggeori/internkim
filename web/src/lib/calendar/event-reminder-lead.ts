import leads from './event-reminder-leads.json';

const minutesInAnHour = 60;
const minutesInADay = 60 * 24;

export const eventReminderLeads: readonly number[] = leads.minutesBeforeStart;

export const defaultEventReminderLead = 30;

export function isEventReminderLead(minutesBeforeStart: number): boolean {
	return eventReminderLeads.includes(minutesBeforeStart);
}

export function eventReminderLeadOf(value: unknown): number | null {
	if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) return null;
	return value;
}

export function eventReminderLeadLabel(minutesBeforeStart: number, locale: string, template: string): string {
	const { unit, amount } = unitOfLead(minutesBeforeStart);
	const spelled = new Intl.NumberFormat(locale, { style: 'unit', unit, unitDisplay: 'long' }).format(amount);
	return template.replace('{amount}', spelled);
}

function unitOfLead(minutesBeforeStart: number): { unit: 'minute' | 'hour' | 'day'; amount: number } {
	if (minutesBeforeStart % minutesInADay === 0) {
		return { unit: 'day', amount: minutesBeforeStart / minutesInADay };
	}
	if (minutesBeforeStart % minutesInAnHour === 0) {
		return { unit: 'hour', amount: minutesBeforeStart / minutesInAnHour };
	}
	return { unit: 'minute', amount: minutesBeforeStart };
}
