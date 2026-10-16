import type { Locale } from '$lib/i18n/locale.svelte';
import type { HolderKind, PeriodKind } from './company';

type CompanyText = {
	documentsTab: string;
	companyTab: string;
	legal: string;
	metrics: string;
	records: string;
	funding: string;
	proof: string;
	contents: string;
	empty: string;
	scope: string;
	attributeNames: Record<string, string>;
	inProgress: string;
	raise: { label: string; target: string; committed: (amount: string, percent: string) => string; closes: string };
	ownership: {
		title: string;
		asOf: (date: string, shares: string) => string;
		shares: (shares: string) => string;
		afterRound: string;
		kinds: Record<HolderKind, string>;
	};
	highlights: Record<'founded' | 'employees' | 'raised' | 'revenue', string>;
	yearsOperating: (years: number) => string;
	rounds: (count: number) => string;
	documents: (count: number) => string;
	people: (count: number) => string;
	change: (percent: string, kind: PeriodKind) => string;
	turnedPositive: string;
	turnedNegative: string;
	date: string;
	round: string;
	amount: string;
	showAll: (count: number) => string;
	showFewer: string;
	undated: string;
	activity: {
		title: string;
		description: string;
		asOf: (day: string) => string;
		tracked: (people: number) => string;
		teamsShown: (shown: number, total: number) => string;
		contributions: string;
		attendance: (day: string, isToday: boolean) => string;
		board: string;
		showMore: string;
		showLess: string;
		contributionsLabel: (count: number) => string;
		events: (count: number) => string;
		month: (month: number) => string;
		weekdays: string[];
		less: string;
		more: string;
		moreTasks: (count: number) => string;
		hidden: string;
	};
	loadFailed: string;
	website: string;
	email: string;
	fields: Record<string, string>;
	settings: {
		title: string;
		description: string;
		field: string;
		category: string;
		readers: string;
		unlisted: string;
		nobody: string;
		kinds: Record<'profile' | 'metric' | 'record' | 'empty', string>;
	};
	metricNames: Record<string, string>;
	recordCategories: Record<string, string>;
};

export const dataRoomCompanyText: Record<Locale, CompanyText> = {
	ko: {
		documentsTab: '자료',
		companyTab: '회사 소개',
		legal: '법인 정보',
		metrics: '주요 지표',
		records: '연혁',
		funding: '자금 조달',
		proof: '주요 실적',
		contents: '데이터룸 자료',
		empty: '이 범위에서 보여줄 회사 정보가 없습니다.',
		scope: '회사 기록에서 바로 읽어 보여줍니다. 기록이 바뀌면 이 화면도 함께 바뀌며, 열람 권한이 있는 분류의 정보만 표시합니다.',
		attributeNames: { investors: '투자사', leadInvestor: '리드 투자사', valuation: '기업가치', preMoney: '투자 전 기업가치', postMoney: '투자 후 기업가치', instrument: '투자 형태', number: '번호', round: '라운드' },
		inProgress: '진행 중',
		raise: { label: '투자 유치 진행 중', target: '목표 금액', committed: (amount, percent) => `${amount} 확약 · ${percent}`, closes: '마감 예정' },
		ownership: {
			title: '지분 구조',
			asOf: (date, shares) => `${date} 기준 · 완전 희석 ${shares}주`,
			shares: (shares) => `${shares}주`,
			afterRound: '라운드 직후',
			kinds: { founder: '창업자', employee: '임직원', investor: '투자자', option_pool: '스톡옵션 풀', other: '기타' }
		},
		highlights: { founded: '설립', employees: '임직원', raised: '누적 투자 유치', revenue: '매출' },
		yearsOperating: (years) => `${years}년 차`,
		rounds: (count) => `${count}개 라운드`,
		documents: (count) => `자료 ${count}건`,
		people: (count) => `${new Intl.NumberFormat('ko').format(count)}명`,
		change: (percent, kind) => `${{ year: '전년 대비', quarter: '전 분기 대비', month: '전월 대비' }[kind]} ${percent}`,
		turnedPositive: '흑자 전환',
		turnedNegative: '적자 전환',
		date: '날짜',
		round: '구분',
		amount: '금액',
		showAll: (count) => `${count}건 모두 보기`,
		showFewer: '접기',
		undated: '날짜 미상',
		activity: {
			title: '일하는 모습',
			description: '이름과 업무 내용은 일부를 가렸습니다.',
			tracked: (people) => `출퇴근을 기록하는 구성원 ${people}명`,
			teamsShown: (shown, total) => `팀 ${total}개 중 인원이 많은 ${shown}개`,
			asOf: (day) => `${day} 기준`,
			contributions: '지난 1년 동안 등록하거나 완료한 업무',
			attendance: (day, isToday) => (isToday ? '오늘의 근태' : `${day} 근태`),
			board: '최근 업무',
			showMore: '더 보기',
			showLess: '접기',
			contributionsLabel: (count) => `${new Intl.NumberFormat('ko').format(count)}건`,
			events: (count) => `${count}건`,
			month: (month) => `${month}월`,
			weekdays: ['월', '', '수', '', '금', '', ''],
			less: '적음',
			more: '많음',
			moreTasks: (count) => `외 ${count}건`,
			hidden: '가려진 내용'
		},
		loadFailed: '회사 정보를 불러오지 못했습니다.',
		website: '웹사이트',
		email: '이메일',
		settings: {
			title: '회사 소개 공개 범위',
			description: '필드마다 어느 분류의 열람자에게 보일지 정합니다. 그 분류를 읽는 서클과 공유 링크에만 값이 전달됩니다.',
			field: '필드',
			category: '공개 분류',
			readers: '볼 수 있는 서클',
			unlisted: '비공개 (X)',
			nobody: '없음',
			kinds: { profile: '회사 정보', metric: '지표', record: '연혁', empty: '아직 값이 없는 필드' }
		},
		fields: {
			name: '법인명',
			brandName: '브랜드명',
			slogan: '슬로건',
			description: '회사 소개',
			representativeTitle: '대표자 직함',
			legalAttributes: '법적 등록 정보',
			email: '대표 이메일',
			fax: '팩스',
			website: '웹사이트',
			foundedDate: '설립일',
			representative: '대표자',
			employeeCount: '임직원 수',
			activity: '일하는 모습 (근태·업무, 이름 일부 가림)',
			ownership: '지분 구조 (보유자 유형별 합계)',
			capital: '자본금',
			jurisdiction: '설립 준거지',
			fiscalYearEnd: '결산월',
			address: '본점 주소',
			officeAddress: '사무실 주소',
			phone: '대표 전화',
			bankAccount: '계좌'
		},
		metricNames: {
			annualRevenue: '매출',
			operatingProfit: '영업이익',
			netIncome: '순이익',
			employees: '임직원 수',
			mau: '월간 활성 사용자'
		},
		recordCategories: {
			history: '마일스톤',
			funding: '투자 유치',
			grant: '지원 사업',
			product: '제품',
			certification: '인증',
			ip: '지식재산',
			award: '수상',
			reference: '고객 사례'
		}
	},
	en: {
		documentsTab: 'Documents',
		companyTab: 'Company',
		legal: 'Legal entity',
		metrics: 'Key metrics',
		records: 'Timeline',
		funding: 'Funding',
		proof: 'Track record',
		contents: 'In this data room',
		empty: 'There is no company information in this scope.',
		scope: 'Read straight from the company record, so this page changes when the record does. Shows only what the categories you can read cover.',
		attributeNames: { investors: 'Investors', leadInvestor: 'Lead investor', valuation: 'Valuation', preMoney: 'Pre-money valuation', postMoney: 'Post-money valuation', instrument: 'Instrument', number: 'Number', round: 'Round' },
		inProgress: 'In progress',
		raise: { label: 'Raising now', target: 'Target', committed: (amount, percent) => `${amount} committed · ${percent}`, closes: 'Expected close' },
		ownership: {
			title: 'Ownership',
			asOf: (date, shares) => `As of ${date} · ${shares} shares fully diluted`,
			shares: (shares) => `${shares} shares`,
			afterRound: 'Right after',
			kinds: { founder: 'Founders', employee: 'Employees', investor: 'Investors', option_pool: 'Option pool', other: 'Other' }
		},
		highlights: { founded: 'Founded', employees: 'Employees', raised: 'Total raised', revenue: 'Revenue' },
		yearsOperating: (years) => `Year ${years}`,
		rounds: (count) => `${count} ${count === 1 ? 'round' : 'rounds'}`,
		documents: (count) => `${count} ${count === 1 ? 'document' : 'documents'}`,
		people: (count) => new Intl.NumberFormat('en').format(count),
		change: (percent, kind) => `${percent} ${{ year: 'YoY', quarter: 'QoQ', month: 'MoM' }[kind]}`,
		turnedPositive: 'Turned positive',
		turnedNegative: 'Turned negative',
		date: 'Date',
		round: 'Type',
		amount: 'Amount',
		showAll: (count) => `Show all ${count}`,
		showFewer: 'Show fewer',
		undated: 'Undated',
		activity: {
			title: 'At work',
			description: 'Names and task details are partly hidden.',
			tracked: (people) => `${people} people record attendance`,
			teamsShown: (shown, total) => `The ${shown} largest of ${total} teams`,
			asOf: (day) => `As of ${day}`,
			contributions: 'Tasks added or completed over the past year',
			attendance: (day, isToday) => (isToday ? 'Attendance today' : `Attendance on ${day}`),
			board: 'Recent work',
			showMore: 'Show more',
			showLess: 'Show less',
			contributionsLabel: (count) => new Intl.NumberFormat('en').format(count),
			events: (count) => `${count} ${count === 1 ? 'task' : 'tasks'}`,
			month: (month) => new Intl.DateTimeFormat('en', { month: 'short', timeZone: 'UTC' }).format(Date.UTC(2000, month - 1, 1)),
			weekdays: ['Mon', '', 'Wed', '', 'Fri', '', ''],
			less: 'Less',
			more: 'More',
			moreTasks: (count) => `${count} more`,
			hidden: 'hidden'
		},
		loadFailed: 'The company information could not be loaded.',
		website: 'Website',
		email: 'Email',
		settings: {
			title: 'Company page visibility',
			description: 'Choose which category each field belongs to. Only the circles and share links that read that category receive its value.',
			field: 'Field',
			category: 'Category',
			readers: 'Circles that see it',
			unlisted: 'Private (X)',
			nobody: 'None',
			kinds: { profile: 'Company', metric: 'Metrics', record: 'Timeline', empty: 'Fields without a value yet' }
		},
		fields: {
			name: 'Legal name',
			brandName: 'Brand',
			slogan: 'Slogan',
			description: 'Description',
			representativeTitle: 'Representative title',
			legalAttributes: 'Legal registration',
			email: 'Email',
			fax: 'Fax',
			website: 'Website',
			foundedDate: 'Founded',
			representative: 'Representative',
			employeeCount: 'Employees',
			activity: 'At work (attendance and tasks, names partly hidden)',
			ownership: 'Ownership (summed by kind of holder)',
			capital: 'Paid-in capital',
			jurisdiction: 'Jurisdiction',
			fiscalYearEnd: 'Fiscal year end',
			address: 'Registered address',
			officeAddress: 'Office',
			phone: 'Phone',
			bankAccount: 'Bank account'
		},
		metricNames: {
			annualRevenue: 'Revenue',
			operatingProfit: 'Operating profit',
			netIncome: 'Net income',
			employees: 'Employees',
			mau: 'Monthly active users'
		},
		recordCategories: {
			history: 'Milestone',
			funding: 'Funding',
		proof: 'Track record',
			grant: 'Grant',
			product: 'Product',
			certification: 'Certification',
			ip: 'IP',
			award: 'Award',
			reference: 'Reference'
		}
	}
};
