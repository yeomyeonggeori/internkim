import type { AttendanceAbsence, AttendanceSummary } from '../attendance-context.svelte';
import type { AttendanceText } from '../text';
import { absenceLabelText } from '../shared/attendance-absence';
import { addDays } from '../shared/attendance-date';
import type { DayHeatCell } from '../shared/attendance-aggregation';

export type TeamCalendarAbsenceTone = 'leave' | 'work' | 'other';

export const teamCalendarVisibleLaneCount = 2;

export type TeamCalendarAbsence = {
	key: string;
	label: string;
	continuationLabel: string;
	periodLabel: string;
	tone: TeamCalendarAbsenceTone;
	lane: number;
	isVisible: boolean;
	starts: boolean;
	ends: boolean;
};

export type TeamCalendarDayCell = DayHeatCell & {
	inCurrentMonth: boolean;
};

export function buildTeamCalendarCells(cells: DayHeatCell[], month: string): TeamCalendarDayCell[] {
	if (!cells.length) return [];
	const byDate = new Map(cells.map((cell) => [cell.date, cell]));
	const firstDate = `${month}-01`;
	const firstDay = new Date(`${firstDate}T00:00:00Z`);
	if (Number.isNaN(firstDay.getTime())) return [];
	const lastDayInMonth = new Date(Date.UTC(firstDay.getUTCFullYear(), firstDay.getUTCMonth() + 1, 0)).getUTCDate();
	const startDate = addDays(firstDate, -firstDay.getUTCDay());
	const totalCells = Math.ceil((firstDay.getUTCDay() + lastDayInMonth) / 7) * 7;
	const totalPeople = cells[0]?.totalPeople ?? 0;
	return Array.from({ length: totalCells }, (_, index) => {
		const date = addDays(startDate, index);
		const current = byDate.get(date);
		if (current) return { ...current, inCurrentMonth: true };
		return { date, presentCount: 0, totalPeople, level: 0, inCurrentMonth: false };
	});
}

export function buildTeamAbsenceByDate(
	summary: AttendanceSummary | null,
	text: Pick<
		AttendanceText,
		'absenceKindLeave' | 'absenceKindBusinessTrip' | 'absenceKindOther'
	>
): Map<string, TeamCalendarAbsence[]> {
	if (!summary) return new Map();
	const namesByEmail = buildTeamCalendarNamesByEmail(summary);
	const byDate = new Map<string, TeamCalendarAbsenceSeed[]>();
	for (const absence of summary.absences) {
		if (absence.canceledAt) continue;
		const labelText = absenceLabelText(absence, text);
		const displayName = namesByEmail.get(absence.email) ?? absence.email;
		const key = absenceSpanKey(absence);
		const list = byDate.get(absence.date) ?? [];
		list.push({
			key,
			label: `${displayName} ${labelText}`,
			continuationLabel: labelText,
			periodLabel: absencePeriodLabel(absence),
			tone: absenceTone(absence.kind),
			createdAt: absence.createdAt,
			starts: absence.isChunkStart,
			ends: absence.isChunkEnd,
		});
		byDate.set(absence.date, list);
	}
	return buildAbsenceSpanMap(byDate);
}

function buildTeamCalendarNamesByEmail(summary: AttendanceSummary): Map<string, string> {
	const names = new Map<string, string>();
	for (const event of summary.events) {
		names.set(event.email, event.displayName || event.mattermostUsername || event.email);
	}
	for (const absence of summary.absences) {
		if (!names.has(absence.email)) names.set(absence.email, fallbackDisplayName(absence.email));
	}
	return names;
}

type TeamCalendarAbsenceSeed = {
	key: string;
	label: string;
	continuationLabel: string;
	periodLabel: string;
	tone: TeamCalendarAbsenceTone;
	createdAt: string;
	starts?: boolean;
	ends?: boolean;
};

type TeamCalendarAbsenceSpan = Pick<
	TeamCalendarAbsenceSeed,
	'key' | 'label' | 'continuationLabel' | 'periodLabel' | 'tone' | 'createdAt'
> & {
	dates: string[];
	spanLength: number;
};

type TeamCalendarPositionedAbsence = TeamCalendarAbsenceSeed & Pick<TeamCalendarAbsence, 'lane' | 'isVisible'>;

function buildAbsenceSpanMap(byDate: Map<string, TeamCalendarAbsenceSeed[]>): Map<string, TeamCalendarAbsence[]> {
	const result = new Map<string, TeamCalendarAbsence[]>();
	const lanes = assignAbsenceLanes(buildAbsenceSpans(byDate));
	for (const [date, list] of byDate.entries()) {
		const sorted = list
			.map((absence) => {
				const lane = lanes.get(absence.key) ?? 0;
				return {
					...absence,
					lane,
					isVisible: lane < teamCalendarVisibleLaneCount,
				};
			})
			.toSorted(compareTeamCalendarAbsences);
		result.set(
			date,
			sorted.map((absence) => ({
				...absence,
				starts: absence.starts ?? !hasAbsenceOn(addDays(date, -1), absence.key, byDate),
				ends: absence.ends ?? !hasAbsenceOn(addDays(date, 1), absence.key, byDate),
			}))
		);
	}
	return result;
}

function buildAbsenceSpans(byDate: Map<string, TeamCalendarAbsenceSeed[]>): TeamCalendarAbsenceSpan[] {
	const spansByKey = new Map<string, TeamCalendarAbsenceSpan>();
	for (const [date, list] of byDate.entries()) {
		for (const absence of list) {
			const span = spansByKey.get(absence.key) ?? {
				key: absence.key,
				label: absence.label,
				continuationLabel: absence.continuationLabel,
				periodLabel: absence.periodLabel,
				tone: absence.tone,
				createdAt: absence.createdAt,
				dates: [],
				spanLength: 0,
			};
			span.dates.push(date);
			span.spanLength = span.dates.length;
			spansByKey.set(absence.key, span);
		}
	}
	return Array.from(spansByKey.values())
		.map((span) => ({
			...span,
			dates: span.dates.toSorted(),
			spanLength: new Set(span.dates).size,
		}))
		.toSorted(compareTeamCalendarAbsenceSpans);
}

function assignAbsenceLanes(spans: TeamCalendarAbsenceSpan[]): Map<string, number> {
	const occupiedDatesByLane: Set<string>[] = [];
	const lanesByKey = new Map<string, number>();
	for (const span of spans) {
		const lane = findAvailableLane(span.dates, occupiedDatesByLane);
		const occupiedDates = occupiedDatesByLane[lane] ?? new Set<string>();
		for (const date of span.dates) {
			occupiedDates.add(date);
		}
		occupiedDatesByLane[lane] = occupiedDates;
		lanesByKey.set(span.key, lane);
	}
	return lanesByKey;
}

function findAvailableLane(dates: string[], occupiedDatesByLane: Set<string>[]): number {
	const lane = occupiedDatesByLane.findIndex((occupiedDates) => !dates.some((date) => occupiedDates.has(date)));
	return lane >= 0 ? lane : occupiedDatesByLane.length;
}

function compareTeamCalendarAbsenceSpans(
	left: TeamCalendarAbsenceSpan,
	right: TeamCalendarAbsenceSpan
): number {
	const spanDifference = right.spanLength - left.spanLength;
	if (spanDifference !== 0) return spanDifference;
	const createdAtDifference = left.createdAt.localeCompare(right.createdAt);
	if (createdAtDifference !== 0) return createdAtDifference;
	return left.key.localeCompare(right.key);
}

function compareTeamCalendarAbsences(
	left: TeamCalendarPositionedAbsence,
	right: TeamCalendarPositionedAbsence
): number {
	const laneDifference = left.lane - right.lane;
	if (laneDifference !== 0) return laneDifference;
	return left.key.localeCompare(right.key);
}

function absenceSpanKey(absence: AttendanceAbsence): string {
	if (absence.rangeID) return absence.rangeID;
	return [absence.email, absence.kind, absence.reason ?? ''].join(':');
}

function absenceTone(kind: AttendanceAbsence['kind']): TeamCalendarAbsenceTone {
	if (kind === 'business_trip') return 'work';
	if (kind === 'other') return 'other';
	return 'leave';
}

function absencePeriodLabel(absence: AttendanceAbsence): string {
	const startDate = absence.startDate ?? absence.date;
	const endDate = absence.endDate ?? absence.date;
	const startLabel = shortDateLabel(startDate);
	const endLabel = shortDateLabel(endDate);
	return startDate === endDate ? startLabel : `${startLabel}~${endLabel}`;
}

function shortDateLabel(date: string): string {
	const [, month, day] = date.split('-');
	return `${Number(month)}/${Number(day)}`;
}

function hasAbsenceOn(date: string, key: string, byDate: Map<string, TeamCalendarAbsenceSeed[]>): boolean {
	return byDate.get(date)?.some((absence) => absence.key === key) ?? false;
}

function fallbackDisplayName(email: string): string {
	return email.split('@')[0] || email;
}
