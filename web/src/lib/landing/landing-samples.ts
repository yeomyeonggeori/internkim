import type { TeamCardStatus } from '../../routes/attendance/team/team-card-status';
import type { Task } from '../../routes/task/task-types';
import type { CompanyMetric, DataRoomCompany } from '$lib/data-room/company';
import type { Locale } from '$lib/i18n/locale';
import { colorPickerPalette } from '$lib/color-picker-palette';
import { startOfCalendarGridWeek } from '../../routes/calendar/grid/calendar-grid-dates';
import type { CalendarGridEvent } from '../../routes/calendar/grid/calendar-grid-layout';

export type SamplePerson = { name: string };

export type SampleTeam = {
	name: string;
	memberCount: number;
	working: number;
	done: number;
	away: number;
	notStarted: number;
	recordedLocations: { name: string; count: number }[];
	unknownLocationCount: number;
	preview: Record<TeamCardStatus, SamplePerson[]>;
};

const peopleNames = {
	ko: { lee: '이샘플', park: '박예시', choi: '최견본', kim: '김예시', han: '한샘플', jung: '정예시' },
	en: { lee: 'Lee Sample', park: 'Park Example', choi: 'Choi Specimen', kim: 'Kim Example', han: 'Han Sample', jung: 'Jung Example' }
};

const words = {
	ko: { office: '사무실', remote: '재택', product: '제품팀', design: '디자인팀', sales: '영업', build: '개발', meeting: '회의', review: '검토', document: '문서', operations: '운영' },
	en: { office: 'Office', remote: 'Remote', product: 'Product', design: 'Design', sales: 'Sales', build: 'Build', meeting: 'Meeting', review: 'Review', document: 'Document', operations: 'Operations' }
};

export function sampleTeams(locale: Locale): SampleTeam[] {
	const person = peopleNames[locale];
	const word = words[locale];
	return [
		{
			name: word.product,
			memberCount: 3,
			working: 3,
			done: 0,
			away: 0,
			notStarted: 0,
			recordedLocations: [{ name: word.office, count: 2 }, { name: word.remote, count: 1 }],
			unknownLocationCount: 0,
			preview: { working: [{ name: person.lee }, { name: person.park }, { name: person.jung }], done: [], away: [], notStarted: [] }
		},
		{
			name: word.design,
			memberCount: 3,
			working: 1,
			done: 0,
			away: 1,
			notStarted: 1,
			recordedLocations: [{ name: word.office, count: 1 }],
			unknownLocationCount: 0,
			preview: { working: [{ name: person.choi }], done: [], away: [{ name: person.kim }], notStarted: [{ name: person.han }] }
		}
	];
}

export function sampleAttendanceSummary(teams: SampleTeam[]) {
	const total = (pick: (team: SampleTeam) => number) => teams.reduce((sum, team) => sum + pick(team), 0);
	return {
		memberCount: total((team) => team.memberCount),
		working: total((team) => team.working),
		done: total((team) => team.done),
		away: total((team) => team.away),
		notStarted: total((team) => team.notStarted),
		needsCheckout: 0
	};
}

const taskContents = {
	ko: ['예시상사 견적서 회신', '신규 고객 미팅 준비', '10월 마감 보고', '랜딩 시안 리뷰', '투자사 자료 정리', '채용 공고 게시', '제품 소개서 개정', '11월 근태 정리'],
	en: ['Reply to Example Co. on the quote', 'Prepare the new client meeting', 'October close report', 'Review the landing draft', 'Gather investor materials', 'Post the job opening', 'Revise the product brochure', 'Wrap up November attendance']
};

export function sampleTasks(locale: Locale, today: Date): Task[] {
	const person = peopleNames[locale];
	const word = words[locale];
	const content = taskContents[locale];
	const day = (offset: number) => isoDate(addDays(today, offset));
	const task = (id: string, status: string, contentIndex: number, owner: string, business: string, type: string, size: string, start: number, end: number): Task => ({
		id,
		ownerID: owner,
		ownerName: owner,
		participantIDs: [owner],
		participantNames: [owner],
		business,
		type,
		content: content[contentIndex],
		size,
		status,
		startDate: day(start),
		endDate: day(end),
		weekCode: ''
	});
	return [
		task('requested-attendance', 'requested', 7, person.han, word.operations, word.document, 'S', 3, 4),
		task('planned-quote', 'planned', 0, person.park, word.sales, word.review, 'S', 1, 2),
		task('planned-meeting', 'planned', 1, person.lee, word.sales, word.meeting, 'S', 2, 2),
		task('progress-close', 'in_progress', 2, person.kim, word.build, word.review, 'M', -2, 1),
		task('progress-landing', 'in_progress', 3, person.choi, word.build, word.review, 'M', -1, 3),
		task('progress-materials', 'in_progress', 4, person.lee, word.sales, word.document, 'L', -3, 2),
		task('completed-opening', 'completed', 5, person.han, word.build, word.document, 'S', -4, -1),
		task('completed-brochure', 'completed', 6, person.jung, word.sales, word.document, 'M', -5, -2)
	];
}

const companyText = {
	ko: {
		name: '샘플컴퍼니',
		slogan: '작은 팀의 일을 대신 처리합니다.',
		round: '시리즈 A',
		instrument: '상환전환우선주',
		currency: 'KRW',
		monthlyRevenue: [35_000_000, 37_000_000, 40_000_000, 42_000_000, 45_000_000, 48_000_000],
		goal: 3_000_000_000,
		committed: 1_800_000_000,
		preMoney: 12_000_000_000
	},
	en: {
		name: 'Sample Company',
		slogan: 'We take the busywork off small teams.',
		round: 'Series A',
		instrument: 'Convertible preferred',
		currency: 'USD',
		monthlyRevenue: [26_000, 27_500, 29_500, 31_000, 33_500, 35_500],
		goal: 2_200_000,
		committed: 1_300_000,
		preMoney: 9_000_000
	}
};

export function sampleCompany(locale: Locale, today: Date): DataRoomCompany {
	const text = companyText[locale];
	const money = (amount: number) => ({ amount, currency: text.currency });
	const months = text.monthlyRevenue.map((_, index) => monthOf(today, index - text.monthlyRevenue.length));
	return {
		profile: { name: { [locale]: text.name }, slogan: { [locale]: text.slogan }, email: 'hello@example.com', website: 'https://example.com' },
		metrics: [
			...text.monthlyRevenue.map((amount, index): CompanyMetric => ({ metric: 'monthlyRevenue', period: months[index], amount: money(amount), basis: 'reported' })),
			{ metric: 'employees', period: months[0], value: 6, basis: 'reported' },
			{ metric: 'employees', period: months[months.length - 1], value: 8, basis: 'reported' }
		],
		records: [
			{
				category: 'funding',
				date: isoDate(addDays(today, 57)),
				title: text.round,
				status: 'in_progress',
				amount: money(text.goal),
				committed: money(text.committed),
				attributes: { preMoney: money(text.preMoney), instrument: text.instrument }
			}
		]
	};
}

function addDays(date: Date, days: number): Date {
	const shifted = new Date(date);
	shifted.setDate(shifted.getDate() + days);
	return shifted;
}

function isoDate(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function monthOf(today: Date, offset: number): string {
	const month = new Date(today.getFullYear(), today.getMonth() + offset, 1);
	return `${month.getFullYear()}-${String(month.getMonth() + 1).padStart(2, '0')}`;
}

const eventTitles = {
	ko: { weekly: '주간 회의', client: '예시상사 미팅', investor: '투자사 미팅', lunch: '팀 점심' },
	en: { weekly: 'Weekly meeting', client: 'Example Co. meeting', investor: 'Investor meeting', lunch: 'Team lunch' }
};

export function sampleEvents(locale: Locale, today: Date): CalendarGridEvent[] {
	const title = eventTitles[locale];
	const person = peopleNames[locale];
	const weekStart = startOfCalendarGridWeek(today);
	const at = (dayOffset: number, hour: number) => {
		const moment = addDays(weekStart, dayOffset);
		moment.setHours(hour, 0, 0, 0);
		return moment;
	};
	const event = (id: string, eventTitle: string, dayOffset: number, startHour: number, endHour: number, color: string, names: string[]): CalendarGridEvent => ({
		id,
		title: eventTitle,
		start: at(dayOffset, startHour),
		end: at(dayOffset, endHour),
		isAllDay: false,
		color,
		participants: names.map((name) => ({ personID: name, name })),
		readOnly: true
	});
	return [
		event('weekly', title.weekly, 2, 10, 11, colorPickerPalette[18], [person.lee, person.park, person.choi]),
		event('client', title.client, 3, 14, 15, colorPickerPalette[21], [person.lee, person.park]),
		event('lunch', title.lunch, 4, 12, 13, colorPickerPalette[12], [person.lee, person.kim, person.han]),
		event('investor', title.investor, 5, 16, 17, colorPickerPalette[15], [person.lee])
	];
}
