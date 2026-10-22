import { defaultEventEndDate } from '$lib/calendar/default-event-duration';
import { currentCRMDate } from './crm-date';

export type CRMActivityDates = {
	isWholeDay: boolean;
	startsAt: string;
	endsAt: string;
};

type HeldDates = {
	isWholeDay?: boolean;
	startsAt?: string;
	endsAt?: string;
	occurredAt: string;
};

const dayLength = 10;
const timeStart = 11;
const timeEnd = 16;

export function localDateTimeValue(date: Date): string {
	if (Number.isNaN(date.getTime())) return '';
	const offset = date.getTimezoneOffset() * 60000;
	return new Date(date.getTime() - offset).toISOString().slice(0, timeEnd);
}

export function todaysActivityDates(now = new Date()): CRMActivityDates {
	const today = currentCRMDate(now);
	return { isWholeDay: true, startsAt: today, endsAt: today };
}

export function activityDatesOf(activity: HeldDates, now = new Date()): CRMActivityDates {
	const isWholeDay = activity.isWholeDay ?? true;
	if (isWholeDay) {
		const startsAt = activity.startsAt?.slice(0, dayLength) || currentCRMDate(now);
		return { isWholeDay, startsAt, endsAt: activity.endsAt?.slice(0, dayLength) || startsAt };
	}
	const startsAt = localDateTimeValue(new Date(activity.startsAt || activity.occurredAt));
	const endsAt = activity.endsAt ? localDateTimeValue(new Date(activity.endsAt)) : '';
	return { isWholeDay, startsAt, endsAt: endsAt || startsAt };
}

export function activityDatesTurned(dates: CRMActivityDates, isWholeDay: boolean, now = new Date()): CRMActivityDates {
	if (isWholeDay === dates.isWholeDay) return dates;
	if (isWholeDay) {
		return { isWholeDay, startsAt: dates.startsAt.slice(0, dayLength), endsAt: dates.endsAt.slice(0, dayLength) };
	}
	const time = localDateTimeValue(now).slice(timeStart, timeEnd);
	return {
		isWholeDay,
		startsAt: `${dates.startsAt}T${time}`,
		endsAt: localDateTimeValue(defaultEventEndDate(new Date(`${dates.endsAt || dates.startsAt}T${time}`)))
	};
}

export function activityDatesAreOrdered(dates: CRMActivityDates): boolean {
	return !dates.startsAt || !dates.endsAt || dates.endsAt >= dates.startsAt;
}

export function activityDatesWritten(dates: CRMActivityDates): CRMActivityDates {
	if (dates.isWholeDay) return { isWholeDay: true, startsAt: dates.startsAt, endsAt: dates.endsAt || dates.startsAt };
	const endsAt = dates.endsAt || dates.startsAt;
	return {
		isWholeDay: false,
		startsAt: dates.startsAt ? new Date(dates.startsAt).toISOString() : '',
		endsAt: endsAt ? new Date(endsAt).toISOString() : ''
	};
}
