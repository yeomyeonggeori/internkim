import type { AttendanceAbsence } from './src/routes/attendance/attendance-context.svelte';

type AbsenceFixtureRange = {
	id: string;
	email: string;
	kind: AttendanceAbsence['kind'];
	startDay: number;
	endDay?: number;
	reason?: string;
	createdBy?: string;
};

type BuildAttendanceAbsenceFixtureOptions = {
	month: string;
	todayDate: string;
	todayMonth: string;
	lastDayInMonth: number;
	todayDay: number;
};

export function buildAttendanceAbsenceFixtures(options: BuildAttendanceAbsenceFixtureOptions): AttendanceAbsence[] {
	const { month, todayDate, todayMonth, lastDayInMonth, todayDay } = options;
	const absences: AttendanceAbsence[] = [];
	const appendAbsence = createAbsenceAppender(absences);
	const appendAbsenceRange = createAbsenceRangeAppender(month, lastDayInMonth, appendAbsence);

	appendAbsence({
		id: 'absence-personal-leave',
		email: 'kim@example.com',
		kind: 'leave',
		date: month === todayMonth ? `${month}-${pad(Math.min(lastDayInMonth, todayDay + 1))}` : `${month}-04`,
		reason: 'family',
		createdBy: 'kim@example.com',
	});
	if (month === '2026-07') {
		appendAbsence({
			id: 'absence-lee-partial-leave',
			email: 'lee@example.com',
			kind: 'leave',
			date: '2026-07-17',
			startTime: '13:00',
			endTime: '15:00',
			reason: 'private appointment',
			createdBy: 'lee@example.com'
		});
	}
	defaultAbsenceRanges().forEach(appendAbsenceRange);
	if (month === '2026-06') juneAbsenceRanges().forEach(appendAbsenceRange);

	return absences;
}

function createAbsenceAppender(absences: AttendanceAbsence[]) {
	return function appendAbsence(absence: Omit<AttendanceAbsence, 'labelKey' | 'createdAt'>): void {
		if (absences.some((existingAbsence) => existingAbsence.email === absence.email && existingAbsence.date === absence.date)) return;
		absences.push({
			...absence,
			rangeID: absence.rangeID ?? absence.id,
			startDate: absence.startDate ?? absence.date,
			endDate: absence.endDate ?? absence.date,
			labelKey: absence.kind,
			createdAt: `${absence.date}T09:00:00+09:00`,
			isRangeStart: absence.isRangeStart ?? true,
			isRangeEnd: absence.isRangeEnd ?? true,
			isChunkStart: absence.isChunkStart ?? true,
			isChunkEnd: absence.isChunkEnd ?? true,
		});
	};
}

function createAbsenceRangeAppender(
	month: string,
	lastDayInMonth: number,
	appendAbsence: (absence: Omit<AttendanceAbsence, 'labelKey' | 'createdAt'>) => void
) {
	return function appendAbsenceRange(range: AbsenceFixtureRange): void {
		const endDay = Math.min(range.endDay ?? range.startDay, lastDayInMonth);
		const startDay = Math.max(1, range.startDay);
		const rangeDates = weekdayDatesInRange(month, startDay, endDay);
		const rangeDateSet = new Set(rangeDates);

		for (const date of rangeDates) {
			appendAbsence({
				id: `${range.id}__date_${date}`,
				rangeID: range.id,
				email: range.email,
				kind: range.kind,
				date,
				startDate: `${month}-${pad(startDay)}`,
				endDate: `${month}-${pad(endDay)}`,
				reason: range.reason,
				createdBy: range.createdBy,
				isRangeStart: date === rangeDates[0],
				isRangeEnd: date === rangeDates[rangeDates.length - 1],
				isChunkStart: !rangeDateSet.has(addFixtureDays(date, -1)),
				isChunkEnd: !rangeDateSet.has(addFixtureDays(date, 1)),
			});
		}
	};
}

function defaultAbsenceRanges(): AbsenceFixtureRange[] {
	return [
		{ id: 'absence-park-vacation', email: 'park@example.com', kind: 'leave', startDay: 6, endDay: 8, reason: 'summer break' },
		{ id: 'absence-jung-other', email: 'jung@example.com', kind: 'other', startDay: 18 },
		{ id: 'absence-kang-other', email: 'kang@example.com', kind: 'other', startDay: 22, endDay: 23, reason: 'personal schedule' },
		{ id: 'absence-lee-short-leave', email: 'lee@example.com', kind: 'leave', startDay: 27, reason: 'family event' },
	];
}

function juneAbsenceRanges(): AbsenceFixtureRange[] {
	return [
		{ id: 'absence-june-kim-leave', email: 'kim@example.com', kind: 'leave', startDay: 9, endDay: 11, reason: 'sample overlap leave' },
		{ id: 'absence-june-choi-other', email: 'choi@example.com', kind: 'other', startDay: 10, reason: 'sample personal schedule' },
		{ id: 'absence-june-choi-leave', email: 'choi@example.com', kind: 'leave', startDay: 24, endDay: 26, reason: 'sample visible range' },
		{ id: 'absence-june-lee-other', email: 'lee@example.com', kind: 'other', startDay: 24, endDay: 26, reason: 'sample overflow range' },
	];
}

function weekdayDatesInRange(month: string, startDay: number, endDay: number): string[] {
	const dates: string[] = [];
	for (let day = startDay; day <= endDay; day += 1) {
		const date = `${month}-${pad(day)}`;
		if (!isWeekendDate(date)) dates.push(date);
	}
	return dates;
}

function isWeekendDate(date: string): boolean {
	const weekday = new Date(`${date}T00:00:00Z`).getUTCDay();
	return weekday === 0 || weekday === 6;
}

function addFixtureDays(date: string, days: number): string {
	const parsedDate = new Date(`${date}T00:00:00Z`);
	parsedDate.setUTCDate(parsedDate.getUTCDate() + days);
	return parsedDate.toISOString().slice(0, 10);
}

function pad(value: number): string {
	return value.toString().padStart(2, '0');
}
