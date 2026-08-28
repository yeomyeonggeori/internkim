import { requiredClockMinute } from './current-work-policy';
import type { DayMinuteInterval } from './leave-day-intervals';
import type { AttendanceWorkPolicyRevision } from './work-calendar-derivation';

export type WorkComplianceVerdict = {
	coreTimeMissed: boolean;
	late: boolean;
	earlyLeave: boolean;
};

const compliant: WorkComplianceVerdict = { coreTimeMissed: false, late: false, earlyLeave: false };

export function workCompliance(
	revision: AttendanceWorkPolicyRevision,
	workingDate: boolean,
	workIntervals: DayMinuteInterval[],
	leaveIntervals: DayMinuteInterval[],
	nowMinute: number
): WorkComplianceVerdict {
	if (!workingDate || revision.workMode === 'autonomous') return compliant;
	const coverage = [...workIntervals, ...leaveIntervals];
	if (revision.workMode === 'flexible') {
		if (!revision.coreTimeEnabled) return compliant;
		const coreStart = clockMinuteOrUndefined(revision.coreStartTime);
		const coreEnd = clockMinuteOrUndefined(revision.coreEndTime);
		if (coreStart === undefined || coreEnd === undefined) return compliant;
		if (nowMinute <= coreEnd) return compliant;
		const required = intervalsExcludingBreaks(coreStart, coreEnd, revision.breakPeriods);
		const coreTimeMissed = required.some((interval) => !intervalCovered(interval, coverage));
		return { coreTimeMissed, late: false, earlyLeave: false };
	}
	const fixedStart = clockMinuteOrUndefined(revision.fixedStartTime);
	const fixedEnd = clockMinuteOrUndefined(revision.fixedEndTime);
	if (fixedStart === undefined || fixedEnd === undefined) return compliant;
	return {
		coreTimeMissed: false,
		late: nowMinute > fixedStart && !minuteCovered(fixedStart, coverage),
		earlyLeave: nowMinute > fixedEnd && !minuteCoveredUpTo(fixedEnd, coverage)
	};
}

export function intervalsExcludingBreaks(
	startMinute: number,
	endMinute: number,
	breakPeriods: { startTime: string; endTime: string }[]
): DayMinuteInterval[] {
	let intervals: DayMinuteInterval[] = [{ startMinute, endMinute }];
	for (const period of breakPeriods) {
		const breakStart = clockMinuteOrUndefined(period.startTime);
		const breakEnd = clockMinuteOrUndefined(period.endTime);
		if (breakStart === undefined || breakEnd === undefined) continue;
		const next: DayMinuteInterval[] = [];
		for (const interval of intervals) {
			if (breakEnd <= interval.startMinute || interval.endMinute <= breakStart) {
				next.push(interval);
				continue;
			}
			if (breakStart > interval.startMinute) {
				next.push({
					startMinute: interval.startMinute,
					endMinute: Math.min(interval.endMinute, breakStart)
				});
			}
			if (interval.endMinute > breakEnd) {
				next.push({
					startMinute: Math.max(interval.startMinute, breakEnd),
					endMinute: interval.endMinute
				});
			}
		}
		intervals = next;
	}
	return intervals;
}

export function intervalCovered(
	required: DayMinuteInterval,
	intervals: DayMinuteInterval[]
): boolean {
	if (required.endMinute <= required.startMinute) return true;
	const sorted = [...intervals].sort((left, right) => left.startMinute - right.startMinute);
	let coveredUntil = required.startMinute;
	for (const interval of sorted) {
		if (interval.endMinute <= coveredUntil || interval.startMinute > coveredUntil) continue;
		coveredUntil = interval.endMinute;
		if (coveredUntil >= required.endMinute) return true;
	}
	return false;
}

function minuteCovered(minute: number, intervals: DayMinuteInterval[]): boolean {
	return intervals.some(
		(interval) => interval.startMinute <= minute && minute < interval.endMinute
	);
}

function minuteCoveredUpTo(minute: number, intervals: DayMinuteInterval[]): boolean {
	return intervals.some(
		(interval) => interval.startMinute < minute && minute <= interval.endMinute
	);
}

function clockMinuteOrUndefined(value: string): number | undefined {
	try {
		return requiredClockMinute(value, 'time');
	} catch {
		return undefined;
	}
}
