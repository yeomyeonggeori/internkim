import type { CurrentAttendanceWorkPolicy } from './current-work-policy';
import {
	companyTimeInstant,
	supabaseWorkStatusTimeRange
} from './supabase-work-status-range';

export type SupabaseWorkEvent = {
	kind: 'clock_in' | 'clock_out';
	occurred_at: string;
};

export type SupabaseWorkedSpan = {
	startAt: Date;
	endAt: Date;
	provisional: boolean;
};

export type SupabaseWorkedDay = {
	spans: SupabaseWorkedSpan[];
	actualMinutes: number;
	actualSeconds: number;
	provisionalMinutes: number;
	provisionalSeconds: number;
	nightMinutes: number;
	isWorking: boolean;
};

export function supabaseWorkedDay(
	events: SupabaseWorkEvent[],
	day: string,
	timeZone: string,
	now: Date,
	policy: CurrentAttendanceWorkPolicy
): SupabaseWorkedDay {
	const dayRange = supabaseWorkStatusTimeRange([day], timeZone);
	const dayStart = new Date(dayRange.from);
	const dayEnd = new Date(dayRange.until);
	const spans = pairedSpans(events, now)
		.map((span) => clippedSpan(span, dayStart, dayEnd))
		.filter((span): span is SupabaseWorkedSpan => span !== null);
	const completed = spans.filter((span) => !span.provisional);
	const provisional = spans.filter((span) => span.provisional);
	return {
		spans,
		actualMinutes: durationMinutes(completed, day, timeZone, policy.breakPeriods),
		actualSeconds: durationSeconds(completed, day, timeZone, policy.breakPeriods),
		provisionalMinutes: durationMinutes(provisional, day, timeZone, policy.breakPeriods),
		provisionalSeconds: durationSeconds(provisional, day, timeZone, policy.breakPeriods),
		nightMinutes: nightMinutes(spans, day, timeZone, policy),
		isWorking: provisional.length > 0
	};
}

function pairedSpans(events: SupabaseWorkEvent[], now: Date): SupabaseWorkedSpan[] {
	const spans: SupabaseWorkedSpan[] = [];
	let openedAt: Date | null = null;
	for (const event of events) {
		if (event.kind === 'clock_in') {
			openedAt = new Date(event.occurred_at);
			continue;
		}
		if (!openedAt) continue;
		spans.push({ startAt: openedAt, endAt: new Date(event.occurred_at), provisional: false });
		openedAt = null;
	}
	if (openedAt) spans.push({ startAt: openedAt, endAt: now, provisional: true });
	return spans;
}

function clippedSpan(
	span: SupabaseWorkedSpan,
	dayStart: Date,
	dayEnd: Date
): SupabaseWorkedSpan | null {
	const startAt = new Date(Math.max(span.startAt.getTime(), dayStart.getTime()));
	const endAt = new Date(Math.min(span.endAt.getTime(), dayEnd.getTime()));
	return endAt > startAt ? { ...span, startAt, endAt } : null;
}

function durationMinutes(
	spans: SupabaseWorkedSpan[],
	day: string,
	timeZone: string,
	breakPeriods: CurrentAttendanceWorkPolicy['breakPeriods']
): number {
	return Math.floor(durationSeconds(spans, day, timeZone, breakPeriods) / 60);
}

function durationSeconds(
	spans: SupabaseWorkedSpan[],
	day: string,
	timeZone: string,
	breakPeriods: CurrentAttendanceWorkPolicy['breakPeriods']
): number {
	return spans.reduce(
		(total, span) => total + spanSecondsExcludingBreaks(span, day, timeZone, breakPeriods),
		0
	);
}

function spanSecondsExcludingBreaks(
	span: SupabaseWorkedSpan,
	day: string,
	timeZone: string,
	breakPeriods: CurrentAttendanceWorkPolicy['breakPeriods']
): number {
	const totalSeconds = overlapSeconds(span.startAt, span.endAt, span.startAt, span.endAt);
	const breakSeconds = breakPeriods.reduce((total, period) => {
		const startAt = new Date(companyTimeInstant(day, period.startTime, timeZone));
		const endAt = new Date(companyTimeInstant(day, period.endTime, timeZone));
		return total + overlapSeconds(span.startAt, span.endAt, startAt, endAt);
	}, 0);
	return Math.max(0, totalSeconds - breakSeconds);
}

function nightMinutes(
	spans: SupabaseWorkedSpan[],
	day: string,
	timeZone: string,
	policy: CurrentAttendanceWorkPolicy
): number {
	const range = supabaseWorkStatusTimeRange([day], timeZone);
	const dayStart = new Date(range.from);
	const dayEnd = new Date(range.until);
	const nightStart = new Date(companyTimeInstant(day, policy.nightStartTime, timeZone));
	const nightEnd = new Date(companyTimeInstant(day, policy.nightEndTime, timeZone));
	const intervals =
		policy.nightStartTime < policy.nightEndTime
			? [[nightStart, nightEnd]]
			: [
					[dayStart, nightEnd],
					[nightStart, dayEnd]
				];
	const seconds = spans.reduce(
		(total, span) =>
			total +
			intervals.reduce((nightTotal, interval) => {
				const nightSpan = clippedSpan(span, interval[0], interval[1]);
				return nightSpan
					? nightTotal + spanSecondsExcludingBreaks(nightSpan, day, timeZone, policy.breakPeriods)
					: nightTotal;
			}, 0),
		0
	);
	return Math.floor(seconds / 60);
}

function overlapSeconds(
	leftStart: Date,
	leftEnd: Date,
	rightStart: Date,
	rightEnd: Date
): number {
	const start = Math.max(leftStart.getTime(), rightStart.getTime());
	const end = Math.min(leftEnd.getTime(), rightEnd.getTime());
	return Math.max(0, Math.floor((end - start) / 1000));
}
