import type { AttendanceAbsence, AttendanceEvent, AttendanceLocation, AttendanceSummary } from '../attendance-context.svelte';
import type { AttendanceText } from '../text';
import { eachDayOfMonth, todayDateInTimeZone } from '../shared/attendance-date';
import { formatHoursMinutes } from '../shared/attendance-format';
import type { AttendanceWorkSegment, PersonToday } from '../shared/attendance-aggregation';
import { computePeopleToday, uniquePeople } from '../shared/attendance-aggregation';

export type TeamStatusPersonDayTone = 'working' | 'finished' | 'absence' | 'absent' | 'empty';
export type TeamStatusAbsenceTone = 'leave' | 'work' | 'other';

export type TeamStatusPersonDaySegment = {
	id: string;
	locationName: string;
	locationID?: string;
	locationColor?: string;
	timeLabel: string;
	durationLabel?: string;
	isOpen: boolean;
};

export type TeamStatusPersonDayAbsenceDetail = {
	label: string;
	periodLabel: string;
	reason?: string;
	createdBy?: string;
};

export type TeamStatusPersonDay = {
	date: string;
	label: string;
	tone: TeamStatusPersonDayTone;
	absenceTone?: TeamStatusAbsenceTone;
	absenceDetail?: TeamStatusPersonDayAbsenceDetail;
	locationName?: string;
	locationID?: string;
	locationColor?: string;
	detailLabel?: string;
	totalDurationLabel?: string;
	segments: TeamStatusPersonDaySegment[];
};

export type TeamStatusPersonRow = Pick<PersonToday, 'email' | 'displayName' | 'mattermostUsername'> & {
	days: TeamStatusPersonDay[];
};

export function buildTeamStatusRows(
	month: string,
	summary: AttendanceSummary,
	text: AttendanceText,
	today: string = todayDateInTimeZone(summary.timeZone)
): TeamStatusPersonRow[] {
	return buildTeamRowsForDates(eachDayOfMonth(month), summary, text, today);
}

export function resolveDefaultDate(
	month: string | undefined,
	events: Pick<AttendanceEvent, 'localDate' | 'kind' | 'canceledAt'>[],
	today: string
): string {
	if (!month) return today;
	if (today.startsWith(month)) return today;
	const dates = events
		.filter((event) => event.kind === 'clock_in' && !event.canceledAt && event.localDate.startsWith(month))
		.map((event) => event.localDate)
		.sort();
	if (dates.length) return dates[0];
	return `${month}-01`;
}

function buildTeamRowsForDates(
	dates: string[],
	summary: AttendanceSummary,
	text: AttendanceText,
	today: string
): TeamStatusPersonRow[] {
	const people = buildTeamPeopleForDates(summary, dates);
	const locationColors = buildLocationColors(summary.locations);
	const peopleByDate = new Map<string, Map<string, PersonToday>>();
	for (const date of dates) {
		const dayPeople = computePeopleToday(date, summary.events, summary.presences, today, summary.absences);
		peopleByDate.set(date, new Map(dayPeople.map((person) => [person.email, person])));
	}
	return people.map((person) => ({
		...person,
		days: dates.map((date) =>
			buildTeamStatusPersonDay(date, peopleByDate.get(date)?.get(person.email), text, locationColors)
		),
	}));
}

function buildTeamPeopleForDates(
	summary: AttendanceSummary,
	dates: string[]
): Pick<PersonToday, 'email' | 'displayName' | 'mattermostUsername'>[] {
	const dateSet = new Set(dates);
	const events = summary.events.filter(
		(event) => event.localDate.startsWith(summary.month) || dateSet.has(event.localDate)
	);
	const absences = summary.absences.filter(
		(absence) => absence.date.startsWith(summary.month) || dateSet.has(absence.date)
	);
	return uniquePeople(events, absences);
}

function buildTeamStatusPersonDay(
	date: string,
	person: PersonToday | undefined,
	text: AttendanceText,
	locationColors: Map<string, string>
): TeamStatusPersonDay {
	if (!person) return emptyTeamStatusPersonDay(date);
	if (person.status === 'working') {
		const locationName = person.activeSegment?.locationName ?? person.locationName;
		const locationID = person.activeSegment?.locationID ?? person.locationID;
		return {
			date,
			label: locationName || text.working,
			tone: 'working',
			locationName,
			locationID,
			locationColor: findLocationColor(locationColors, locationID, locationName),
			detailLabel: person.activeSegment?.startTime,
			totalDurationLabel: person.workedMinutes > 0 ? formatHoursMinutes(person.workedMinutes) : undefined,
			segments: buildTeamStatusPersonDaySegments(person.segments, text, locationColors),
		};
	}
	if (person.status === 'finished') {
		return {
			date,
			label: text.finished,
			tone: 'finished',
			detailLabel: person.workedMinutes > 0 ? formatHoursMinutes(person.workedMinutes) : undefined,
			totalDurationLabel: person.workedMinutes > 0 ? formatHoursMinutes(person.workedMinutes) : undefined,
			segments: buildTeamStatusPersonDaySegments(person.segments, text, locationColors),
		};
	}
	if (person.status === 'absence' && person.absence) {
		const absenceLabel = teamStatusAbsenceLabel(person.absence, text);
		return {
			date,
			label: absenceLabel,
			tone: 'absence',
			absenceTone: absenceTone(person.absence.kind),
			absenceDetail: {
				label: absenceLabel,
				periodLabel: absencePeriodLabel(person.absence),
				reason: person.absence.reason,
				createdBy: person.absence.createdBy,
			},
			segments: [],
		};
	}
	if (person.status === 'absent') return { date, label: text.absent, tone: 'absent', segments: [] };
	return emptyTeamStatusPersonDay(date);
}

function emptyTeamStatusPersonDay(date: string): TeamStatusPersonDay {
	return { date, label: '-', tone: 'empty', segments: [] };
}

function buildTeamStatusPersonDaySegments(
	segments: AttendanceWorkSegment[],
	text: AttendanceText,
	locationColors: Map<string, string>
): TeamStatusPersonDaySegment[] {
	return segments.map((segment) => ({
		id: segment.id,
		locationName: segment.locationName || '-',
		locationID: segment.locationID,
		locationColor: findLocationColor(locationColors, segment.locationID, segment.locationName),
		timeLabel: segment.isOpen ? `${segment.startTime}~` : `${segment.startTime}-${segment.endTime ?? ''}`,
		durationLabel: segment.isOpen
			? text.inProgress
			: segment.workedMinutes > 0
				? formatHoursMinutes(segment.workedMinutes)
				: undefined,
		isOpen: segment.isOpen,
	}));
}

function buildLocationColors(locations: AttendanceLocation[]): Map<string, string> {
	const colors = new Map<string, string>();
	for (const location of locations) {
		if (!location.color) continue;
		colors.set(location.id, location.color);
		colors.set(location.name, location.color);
	}
	return colors;
}

function findLocationColor(
	locationColors: Map<string, string>,
	locationID: string | undefined,
	locationName: string | undefined
): string | undefined {
	if (locationID && locationColors.has(locationID)) return locationColors.get(locationID);
	if (locationName && locationColors.has(locationName)) return locationColors.get(locationName);
	return undefined;
}

function teamStatusAbsenceLabel(
	absence: AttendanceAbsence,
	text: Pick<AttendanceText, 'absenceKindLeave' | 'absenceKindBusinessTrip' | 'absenceKindOther'>
): string {
	if (absence.kind === 'leave') return text.absenceKindLeave;
	if (absence.kind === 'business_trip') return text.absenceKindBusinessTrip;
	return text.absenceKindOther;
}

function absenceTone(kind: AttendanceAbsence['kind']): TeamStatusAbsenceTone {
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
