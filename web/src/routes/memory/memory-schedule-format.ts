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

export function formatScheduleCronExpression(cronExpression: string, locale: 'ko' | 'en'): string {
	const parsedCronExpression = parseSimpleCronExpression(cronExpression);
	if (!parsedCronExpression) return cronExpression;

	const timeText = formatCronTime(parsedCronExpression.hour, parsedCronExpression.minute, locale);
	if (parsedCronExpression.dayOfWeek === '*') {
		return locale === 'ko' ? `매일 ${timeText}` : `Every day at ${timeText}`;
	}
	if (parsedCronExpression.dayOfWeek === '1-5') {
		return locale === 'ko' ? `평일 ${timeText}` : `Weekdays at ${timeText}`;
	}
	const weekDayText = cronWeekDayText(parsedCronExpression.dayOfWeek, locale);
	if (weekDayText) {
		return locale === 'ko' ? `매주 ${weekDayText} ${timeText}` : `Every ${weekDayText} at ${timeText}`;
	}
	return cronExpression;
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
	if (locale === 'en') {
		const period = hour >= 12 ? 'PM' : 'AM';
		const hour12 = hour % 12 || 12;
		return `${hour12}:${minute.toString().padStart(2, '0')} ${period}`;
	}
	const period = hour >= 12 ? '오후' : '오전';
	const hour12 = hour % 12 || 12;
	return `${period} ${hour12}:${minute.toString().padStart(2, '0')}`;
}

function cronWeekDayText(dayOfWeek: string, locale: 'ko' | 'en'): string | undefined {
	const dayIndex = Number(dayOfWeek);
	if (!Number.isInteger(dayIndex) || dayIndex < 0 || dayIndex > 7) return undefined;
	const normalizedDayIndex = dayIndex === 7 ? 0 : dayIndex;
	const koreanWeekDays = ['일요일', '월요일', '화요일', '수요일', '목요일', '금요일', '토요일'];
	const englishWeekDays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
	return locale === 'ko' ? koreanWeekDays[normalizedDayIndex] : englishWeekDays[normalizedDayIndex];
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
