import type { AttendanceKind, AttendancePresence, AttendanceSummary } from '../../src/routes/attendance/attendance-context.svelte';
import { todayDateInTimeZone } from '../../src/routes/attendance/shared/attendance-date';

type Scenario = 'normal' | 'ambiguous' | 'overridden' | 'manual';

const CLOCK_IN_MESSAGES: Record<string, string[]> = {
	office: ['사무실 도착했어요', '오늘 시작합니다', '사무실 출근'],
	remote: ['오늘 재택할게요', '재택 시작합니다', '집에서 일합니다'],
	bss: ['BSS 도착했어요', 'BSS에서 시작합니다', 'BSS 출근'],
	outside: ['외부 출발합니다', '현장 도착했어요', '외부 일정 시작'],
};

const CLOCK_OUT_MESSAGES = ['퇴근합니다', '오늘은 여기까지', '퇴근할게요'];

const AMBIGUOUS_MESSAGES = [
	'좀 늦어요',
	'오늘 컨디션이 안 좋네요',
	'조금 이따 시작할게요',
	'잠깐 자리 비울게요',
];

const AMBIGUOUS_CLOCK_OUT_MESSAGES = [
	'이제 갈게요',
	'잠시 자리 비울게요',
	'오늘은 일찍 마칠게요',
	'잠깐 외출',
];

function pickMessage(pool: string[], seed: number): string {
	return pool[seed % pool.length];
}

function scenarioFor(day: number, personIndex: number): Scenario {
	const seed = (day * 7 + personIndex * 13) % 20;
	if (seed === 0) return 'manual';
	if (seed === 1) return 'overridden';
	if (seed === 2 || seed === 3) return 'ambiguous';
	return 'normal';
}

type Annotation = {
	sourceMessage?: string;
	confidence?: number;
	parsedAs?: { kind?: AttendanceKind; locationID?: string };
	overriddenBy?: string;
	overriddenAt?: string;
	manualEntry?: boolean;
};

function annotateClockIn(scenario: Scenario, locationID: string, seed: number, date: string): Annotation {
	if (scenario === 'manual') {
		return { manualEntry: true };
	}
	if (scenario === 'ambiguous') {
		return {
			sourceMessage: pickMessage(AMBIGUOUS_MESSAGES, seed),
			confidence: 0.48,
			parsedAs: { kind: 'clock_in', locationID },
		};
	}
	if (scenario === 'overridden') {
		const messagePool = CLOCK_IN_MESSAGES[locationID] ?? CLOCK_IN_MESSAGES.office;
		const initialGuess = locationID === 'office' ? 'remote' : 'office';
		return {
			sourceMessage: pickMessage(messagePool, seed),
			confidence: 0.71,
			parsedAs: { kind: 'clock_in', locationID: initialGuess },
			overriddenBy: 'kim@example.com',
			overriddenAt: `${date}T10:30:00+09:00`,
		};
	}
	const pool = CLOCK_IN_MESSAGES[locationID] ?? CLOCK_IN_MESSAGES.office;
	return {
		sourceMessage: pickMessage(pool, seed),
		confidence: 0.92,
		parsedAs: { kind: 'clock_in', locationID },
	};
}

function annotateClockOut(scenario: Scenario, locationID: string, seed: number, date: string): Annotation {
	if (scenario === 'manual') {
		return { manualEntry: true };
	}
	if (scenario === 'ambiguous') {
		return {
			sourceMessage: pickMessage(AMBIGUOUS_CLOCK_OUT_MESSAGES, seed),
			confidence: 0.5,
			parsedAs: { kind: 'clock_out', locationID },
		};
	}
	if (scenario === 'overridden') {
		return {
			sourceMessage: pickMessage(CLOCK_OUT_MESSAGES, seed),
			confidence: 0.72,
			parsedAs: { kind: 'clock_out', locationID: locationID === 'office' ? 'remote' : 'office' },
			overriddenBy: 'kim@example.com',
			overriddenAt: `${date}T19:00:00+09:00`,
		};
	}
	return {
		sourceMessage: pickMessage(CLOCK_OUT_MESSAGES, seed),
		confidence: 0.93,
		parsedAs: { kind: 'clock_out', locationID },
	};
}

export function buildAttendanceSummaryFixture(month: string): AttendanceSummary {
	const today = new Date();
	const todayDate = todayDateInTimeZone('Asia/Seoul', today);
	const people = [
		{ email: 'kim@example.com', name: '김철수', mm: 'kim', baseHour: 8, baseMinute: 50 },
		{ email: 'lee@example.com', name: '이영희', mm: 'lee', baseHour: 8, baseMinute: 45 },
		{ email: 'park@example.com', name: '박지민', mm: 'park', baseHour: 9, baseMinute: 5 },
		{ email: 'choi@example.com', name: '최민준', mm: 'choi', baseHour: 9, baseMinute: 10 },
		{ email: 'jung@example.com', name: '정수아', mm: 'jung', baseHour: 8, baseMinute: 55 },
		{ email: 'kang@example.com', name: '강민호', mm: 'kang', baseHour: 9, baseMinute: 20 },
	];
	const locations = [
		{ id: 'office', name: '사무실', color: '#22c55e', isDefault: true },
		{ id: 'remote', name: '재택', color: '#3b82f6', isDefault: false },
		{ id: 'bss', name: 'BSS', color: '#0ea5e9', isDefault: false },
		{ id: 'outside', name: '외부', color: '#f59e0b', isDefault: false },
	];
	const presences: Record<string, AttendancePresence> = {
		'kim@example.com': 'online',
		'lee@example.com': 'away',
		'park@example.com': 'online',
		'choi@example.com': 'offline',
		'jung@example.com': 'dnd',
		'kang@example.com': 'offline',
	};

	const events: AttendanceSummary['events'] = [];
	const absences: AttendanceSummary['absences'] = [];
	let idSeed = 0;

	function pickLocation(personIndex: number, day: number) {
		if (day % 11 === 2 && personIndex === 0) return locations[2];
		if (day % 7 === 3 && personIndex >= 4) return locations[3];
		if (day % 5 === 0 && personIndex !== 0) return locations[1];
		return locations[0];
	}

	function pad(value: number) {
		return value.toString().padStart(2, '0');
	}

	const todayMonth = todayDate.slice(0, 7);
	const [yearString, monthString] = month.split('-');
	const yearNumber = Number(yearString);
	const monthNumber = Number(monthString);
	if (!yearNumber || !monthNumber) {
		return {
			month,
			currentUserEmail: 'kim@example.com',
			isAdmin: true,
			timeZone: 'Asia/Seoul',
			events,
			absences,
			todayStatus: '근무 중',
			locations,
			teamViewVisibleToAll: true,
			teamViewBlocked: false,
			presences,
		};
	}
	const lastDayInMonth = new Date(Date.UTC(yearNumber, monthNumber, 0)).getUTCDate();
	let endDay: number;
	if (month === todayMonth) {
		endDay = today.getDate();
	} else if (month < todayMonth) {
		endDay = lastDayInMonth;
	} else {
		endDay = 0;
	}

	const teamAbsenceDate = month === todayMonth ? todayDate : `${month}-01`;
	const personalAbsenceDate = month === todayMonth ? `${month}-${pad(Math.min(lastDayInMonth, today.getDate() + 1))}` : `${month}-04`;
	absences.push(
		{
			id: 'absence-team-business-trip',
			email: 'lee@example.com',
			kind: 'business_trip',
			labelKey: 'business_trip',
			date: teamAbsenceDate,
			createdAt: `${teamAbsenceDate}T09:00:00+09:00`
		},
		{
			id: 'absence-personal-leave',
			email: 'kim@example.com',
			kind: 'leave',
			labelKey: 'leave',
			date: personalAbsenceDate,
			reason: 'family',
			createdBy: 'kim@example.com',
			createdAt: `${personalAbsenceDate}T09:00:00+09:00`
		}
	);

	for (let d = 1; d <= endDay; d++) {
		const date = `${month}-${pad(d)}`;
		const weekday = new Date(`${date}T00:00:00Z`).getUTCDay();
		if (weekday === 0 || weekday === 6) continue;

		people.forEach((person, personIndex) => {
			if (absences.some((absence) => absence.date === date && absence.email === person.email)) return;

			const isAbsentToday = person.email === 'jung@example.com' && d % 3 === 0;
			const isAbsentLate = person.email === 'kang@example.com' && d % 4 === 0;
			if (isAbsentToday || isAbsentLate) return;

			const location = pickLocation(personIndex, d);
			const hourJitter = (d + personIndex) % 3 === 0 ? 1 : 0;
			const minuteJitter = ((d * 13) + personIndex * 7) % 30;
			const hour = person.baseHour + hourJitter;
			const minute = (person.baseMinute + minuteJitter) % 60;
			const carryHour = person.baseMinute + minuteJitter >= 60 ? hour + 1 : hour;

			const scenario = scenarioFor(d, personIndex);
			const messageSeed = d + personIndex;
			const clockInAnnotation = annotateClockIn(scenario, location.id, messageSeed, date);

			const clockInIso = `${date}T${pad(carryHour)}:${pad(minute)}:00+09:00`;
			events.push({
				id: `in-${++idSeed}`,
				mattermostUserID: person.mm,
				mattermostUsername: person.mm,
				email: person.email,
				displayName: person.name,
				kind: 'clock_in',
				occurredAt: clockInIso,
				localDate: date,
				localTime: `${pad(carryHour)}:${pad(minute)}`,
				timeZoneAtEvent: 'Asia/Seoul',
				source: 'mattermost_button',
				resultPostID: `post-${idSeed}`,
				locationID: location.id,
				locationName: location.name,
				...clockInAnnotation,
			});

			const isStillWorkingToday = date === todayDate && personIndex % 2 === 0;
			if (date === todayDate && isStillWorkingToday) return;

			if (d % 9 === 0 && personIndex === 2) {
				events.push({
					id: `in-${++idSeed}`,
					mattermostUserID: person.mm,
					mattermostUsername: person.mm,
					email: person.email,
					displayName: person.name,
					kind: 'clock_in',
					occurredAt: `${date}T${pad(carryHour)}:${pad((minute + 15) % 60)}:00+09:00`,
					localDate: date,
					localTime: `${pad(carryHour)}:${pad((minute + 15) % 60)}`,
					timeZoneAtEvent: 'Asia/Seoul',
					source: 'mattermost_button',
					resultPostID: `post-${idSeed}`,
					locationID: location.id,
					locationName: location.name,
					canceledAt: `${date}T${pad(carryHour)}:${pad((minute + 20) % 60)}:00+09:00`,
					cancelReason: 'duplicate_click',
				});
			}

			const outHour = 17 + (personIndex % 3);
			const outMinute = (personIndex * 11 + d * 3) % 60;
			const outIso = `${date}T${pad(outHour)}:${pad(outMinute)}:00+09:00`;
			const clockOutAnnotation = annotateClockOut(scenario, location.id, messageSeed + 1, date);
			events.push({
				id: `out-${++idSeed}`,
				mattermostUserID: person.mm,
				mattermostUsername: person.mm,
				email: person.email,
				displayName: person.name,
				kind: 'clock_out',
				occurredAt: outIso,
				localDate: date,
				localTime: `${pad(outHour)}:${pad(outMinute)}`,
				timeZoneAtEvent: 'Asia/Seoul',
				source: 'mattermost_button',
				resultPostID: `post-${idSeed}`,
				locationID: location.id,
				locationName: location.name,
				...clockOutAnnotation,
			});
		});
	}

	return {
		month,
		currentUserEmail: 'kim@example.com',
		isAdmin: true,
		timeZone: 'Asia/Seoul',
		events,
		absences,
		todayStatus: '근무 중',
		locations,
		teamViewVisibleToAll: true,
		teamViewBlocked: false,
		presences,
	};
}
