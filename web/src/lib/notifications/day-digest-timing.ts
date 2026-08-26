import { readNotificationSettings } from './categories';

export type Listener = {
	memberID: string;
	timeZone: string;
	notificationSettings: unknown;
};

export function timeOfDayIn(timeZone: string, moment: Date): string {
	return new Intl.DateTimeFormat('en-GB', {
		timeZone,
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	}).format(moment);
}

export function dayIn(timeZone: string, moment: Date): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(moment);
}

export function whoseHourItIs(listeners: Listener[], moment: Date): Listener[] {
	return listeners.filter((listener) => {
		const settings = readNotificationSettings(listener.notificationSettings);
		if (!settings.categories.calendar) return false;
		return settings.calendarAt === timeOfDayIn(listener.timeZone, moment);
	});
}
