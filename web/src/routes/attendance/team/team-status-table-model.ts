import type { AttendanceAbsence, AttendanceEvent, AttendanceLocation, AttendanceMember, AttendanceSummary } from '../attendance-context.svelte';
import type { AttendanceText } from '../text';
import { eachDayOfMonth, isWeekend, todayDateInTimeZone } from '../shared/attendance-date';
import { formatHoursMinutes } from '../shared/attendance-format';
import type { AttendanceWorkSegment, PersonToday } from '../shared/attendance-aggregation';
import { computePeopleToday, uniquePeople } from '../shared/attendance-aggregation';

export type TeamStatusPersonDayTone = 'working' | 'finished' | 'absence' | 'absent' | 'empty';
export type TeamStatusAbsenceTone = 'leave' | 'other';

export type TeamStatusPersonDaySegment = {
	id: string;
	locationName: string;
	locationID?: string;
	locationColor?: string;
	timeLabel: string;
	durationLabel?: string;
	sharePercent: number;
	tooltipLabel: string;
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
	currentLocationName?: string;
	currentLocationColor?: string;
	days: TeamStatusPersonDay[];
};

type LocationColorLookup = {
	byID: Map<string, string>;
	byName: Map<string, string>;
};

export function buildTeamStatusRows(
	month: string,
	summary: AttendanceSummary,
	text: AttendanceText,
	today: string = todayDateInTimeZone(summary.timeZone),
	currentSummary: AttendanceSummary = summary
): TeamStatusPersonRow[] {
	return buildTeamRowsForDates(eachDayOfMonth(month), summary, text, today, currentSummary);
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
	today: string,
	currentSummary: AttendanceSummary
): TeamStatusPersonRow[] {
	const people = buildTeamPeopleForDates(summary, dates);
	const locationColors = buildLocationColors(summary.locations);
	const peopleByDate = new Map<string, Map<string, PersonToday>>();
	for (const date of dates) {
		const dayPeople = computePeopleToday(date, summary.events, summary.presences, today, summary.absences);
		peopleByDate.set(date, new Map(dayPeople.map((person) => [person.email, person])));
	}
	const currentLocationColors = buildLocationColors(currentSummary.locations);
	const todayPeople = new Map(computePeopleToday(
		today,
		currentSummary.events,
		currentSummary.presences,
		today,
		currentSummary.absences
	).map((person) => [person.email, person]));
	return people.map((person) => {
		const todayPerson = todayPeople.get(person.email);
		const currentLocationName = todayPerson?.status === 'working'
			? (todayPerson.activeSegment?.locationName ?? todayPerson.locationName)
			: undefined;
		const currentLocationID = todayPerson?.status === 'working'
			? (todayPerson.activeSegment?.locationID ?? todayPerson.locationID)
			: undefined;
		return {
			...person,
			currentLocationName,
			currentLocationColor: findLocationColor(currentLocationColors, currentLocationID, currentLocationName),
			days: dates.map((date) => {
				const dayPerson = peopleByDate.get(date)?.get(person.email);
				return dayPerson
					? buildTeamStatusPersonDay(date, dayPerson, text, locationColors)
					: fallbackTeamStatusPersonDay(date, today);
			}),
		};
	});
}

function buildTeamPeopleForDates(
	summary: AttendanceSummary,
	dates: string[]
): Pick<PersonToday, 'email' | 'displayName' | 'mattermostUsername'>[] {
	if ((summary.members?.length ?? 0) > 0) {
		return summary.members.map(attendanceMemberToTeamPerson);
	}
	const dateSet = new Set(dates);
	const events = summary.events.filter(
		(event) => event.localDate.startsWith(summary.month) || dateSet.has(event.localDate)
	);
	const absences = summary.absences.filter(
		(absence) => absence.date.startsWith(summary.month) || dateSet.has(absence.date)
	);
	return uniquePeople(events, absences);
}

function attendanceMemberToTeamPerson(member: AttendanceMember): Pick<PersonToday, 'email' | 'displayName' | 'mattermostUsername'> {
	return {
		email: member.email,
		displayName: member.displayName || member.mattermostUsername || member.email,
		mattermostUsername: member.mattermostUsername,
	};
}

function buildTeamStatusPersonDay(
	date: string,
	person: PersonToday | undefined,
	text: AttendanceText,
	locationColors: LocationColorLookup
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
			totalDurationLabel: person.workedMinutes > 0 ? formatHoursMinutes(person.workedMinutes, text) : undefined,
			segments: buildTeamStatusPersonDaySegments(person.segments, text, locationColors),
		};
	}
	if (person.status === 'finished') {
		const totalDurationLabel = person.workedMinutes > 0 ? formatHoursMinutes(person.workedMinutes, text) : undefined;
		return {
			date,
			label: totalDurationLabel ?? '-',
			tone: 'finished',
			totalDurationLabel,
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
	if (person.status === 'absent') return { date, label: '-', tone: 'absent', segments: [] };
	return emptyTeamStatusPersonDay(date);
}

function emptyTeamStatusPersonDay(date: string): TeamStatusPersonDay {
	return { date, label: '-', tone: 'empty', segments: [] };
}

function fallbackTeamStatusPersonDay(date: string, today: string): TeamStatusPersonDay {
	if (!isWeekend(date) && date <= today) return { date, label: '-', tone: 'absent', segments: [] };
	return emptyTeamStatusPersonDay(date);
}

function buildTeamStatusPersonDaySegments(
	segments: AttendanceWorkSegment[],
	text: AttendanceText,
	locationColors: LocationColorLookup
): TeamStatusPersonDaySegment[] {
	const totalWorkedMinutes = segments.reduce((totalMinutes, segment) => totalMinutes + segment.workedMinutes, 0);
	const fallbackSharePercent = segments.length > 0 ? Math.round(100 / segments.length) : 0;
	return segments.map((segment) => {
		const locationName = segment.locationName || '-';
		const timeLabel = segment.isOpen ? `${segment.startTime}~` : `${segment.startTime}-${segment.endTime ?? ''}`;
		const durationLabel = segment.isOpen
			? text.inProgress
			: segment.workedMinutes > 0
				? formatHoursMinutes(segment.workedMinutes, text)
				: undefined;
		const sharePercent = totalWorkedMinutes > 0
			? Math.max(1, Math.round((segment.workedMinutes / totalWorkedMinutes) * 100))
			: fallbackSharePercent;
		return {
			id: segment.id,
			locationName,
			locationID: segment.locationID,
			locationColor: findLocationColor(locationColors, segment.locationID, segment.locationName),
			timeLabel,
			durationLabel,
			sharePercent,
			tooltipLabel: durationLabel ? `${locationName} ${timeLabel} · ${durationLabel}` : `${locationName} ${timeLabel}`,
			isOpen: segment.isOpen,
		};
	});
}

function buildLocationColors(locations: AttendanceLocation[]): LocationColorLookup {
	const byID = new Map<string, string>();
	const byName = new Map<string, string>();
	for (const location of locations) {
		if (!location.color) continue;
		byID.set(location.id, location.color);
		byName.set(location.name, location.color);
	}
	return { byID, byName };
}

function findLocationColor(
	locationColors: LocationColorLookup,
	locationID: string | undefined,
	locationName: string | undefined
): string | undefined {
	if (locationID && locationColors.byID.has(locationID)) return locationColors.byID.get(locationID);
	if (locationName && locationColors.byName.has(locationName)) return locationColors.byName.get(locationName);
	return undefined;
}

function teamStatusAbsenceLabel(
	absence: AttendanceAbsence,
	text: Pick<AttendanceText, 'absenceKindLeave' | 'absenceKindOther'>
): string {
	if (absence.kind === 'leave') return text.absenceKindLeave;
	return text.absenceKindOther;
}

function absenceTone(kind: AttendanceAbsence['kind']): TeamStatusAbsenceTone {
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
