import type { MemoryText } from './text';

export function formatScheduleDateTime(
	value: string | undefined,
	timeZone: string | undefined,
	locale: string,
	fallbackTimeZone = browserTimeZone()
): string | undefined {
	if (!value) return undefined;
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return undefined;
	return new Intl.DateTimeFormat(locale, {
		dateStyle: 'medium',
		timeStyle: 'short',
		timeZone: normalizeScheduleTimeZone(timeZone, fallbackTimeZone)
	}).format(date);
}

export function formatScheduleCronExpression(cronExpression: string, locale: 'ko' | 'en', text: MemoryText): string {
	const parsedCronExpression = parseSimpleCronExpression(cronExpression);
	if (!parsedCronExpression) return cronExpression;

	const timeText = formatCronTime(parsedCronExpression.hour, parsedCronExpression.minute, locale);
	if (parsedCronExpression.dayOfWeek === '*') {
		return fillScheduleTemplate(text.scheduleCronDailyTemplate, { time: timeText });
	}
	if (parsedCronExpression.dayOfWeek === '1-5') {
		return fillScheduleTemplate(text.scheduleCronWeekdaysTemplate, { time: timeText });
	}
	const weekDayText = cronWeekDayText(parsedCronExpression.dayOfWeek, text);
	if (weekDayText) {
		return fillScheduleTemplate(text.scheduleCronWeeklyTemplate, {
			time: timeText,
			weekday: weekDayText
		});
	}
	return cronExpression;
}

export function formatScheduleInterval(intervalSecond: number, text: MemoryText): string {
	if (intervalSecond % 3600 === 0) {
		const hourCount = intervalSecond / 3600;
		return fillCountTemplate(
			hourCount,
			text.scheduleHourIntervalSingularTemplate,
			text.scheduleHourIntervalTemplate
		);
	}
	if (intervalSecond % 60 === 0) {
		const minuteCount = intervalSecond / 60;
		return fillCountTemplate(
			minuteCount,
			text.scheduleMinuteIntervalSingularTemplate,
			text.scheduleMinuteIntervalTemplate
		);
	}
	return fillCountTemplate(
		intervalSecond,
		text.scheduleSecondIntervalSingularTemplate,
		text.scheduleSecondIntervalTemplate
	);
}

type SimpleCronExpression = {
	minute: number;
	hour: number;
	dayOfWeek: string;
};

function parseSimpleCronExpression(cronExpression: string): SimpleCronExpression | undefined {
	const parts = cronExpression.trim().split(/\s+/);
	if (parts.length !== 5) return undefined;
	const [minuteValue, hourValue, dayOfMonth, month, dayOfWeek] = parts;
	if (dayOfMonth !== '*' || month !== '*') return undefined;
	const minute = Number(minuteValue);
	const hour = Number(hourValue);
	if (!Number.isInteger(minute) || minute < 0 || minute > 59) return undefined;
	if (!Number.isInteger(hour) || hour < 0 || hour > 23) return undefined;
	return { minute, hour, dayOfWeek };
}

function formatCronTime(hour: number, minute: number, locale: 'ko' | 'en'): string {
	const date = new Date(Date.UTC(2000, 0, 1, hour, minute));
	return new Intl.DateTimeFormat(locale === 'ko' ? 'ko-KR' : 'en-US', {
		hour: 'numeric',
		minute: '2-digit',
		timeZone: 'UTC'
	}).format(date);
}

function cronWeekDayText(dayOfWeek: string, text: MemoryText): string | undefined {
	const dayIndex = Number(dayOfWeek);
	if (!Number.isInteger(dayIndex) || dayIndex < 0 || dayIndex > 7) return undefined;
	const normalizedDayIndex = dayIndex === 7 ? 0 : dayIndex;
	const weekDays = [
		text.scheduleWeekdaySunday,
		text.scheduleWeekdayMonday,
		text.scheduleWeekdayTuesday,
		text.scheduleWeekdayWednesday,
		text.scheduleWeekdayThursday,
		text.scheduleWeekdayFriday,
		text.scheduleWeekdaySaturday
	];
	return weekDays[normalizedDayIndex];
}

function fillScheduleTemplate(template: string, values: Record<string, string>): string {
	return Object.entries(values).reduce(
		(result, [key, value]) => result.replaceAll(`{${key}}`, value),
		template
	);
}

function fillCountTemplate(count: number, singularTemplate: string, pluralTemplate: string): string {
	return fillScheduleTemplate(count === 1 ? singularTemplate : pluralTemplate, {
		count: String(count)
	});
}

function normalizeScheduleTimeZone(timeZone: string | undefined, fallbackTimeZone: string): string {
	const trimmedTimeZone = timeZone?.trim();
	if (trimmedTimeZone && isValidTimeZone(trimmedTimeZone)) return trimmedTimeZone;
	if (isValidTimeZone(fallbackTimeZone)) return fallbackTimeZone;
	return 'UTC';
}

function browserTimeZone(): string {
	return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}

function isValidTimeZone(timeZone: string): boolean {
	try {
		new Intl.DateTimeFormat('en-US', { timeZone });
		return true;
	} catch {
		return false;
	}
}
