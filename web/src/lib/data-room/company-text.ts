import type { Locale } from '$lib/i18n/locale.svelte';

type CompanyText = {
	documentsTab: string;
	companyTab: string;
	overview: string;
	metrics: string;
	records: string;
	empty: string;
	scope: string;
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
		overview: '회사 개요',
		metrics: '주요 지표',
		records: '연혁',
		empty: '이 범위에서 보여줄 회사 정보가 없습니다.',
		scope: '열람 권한이 있는 분류의 정보만 표시합니다.',
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
		overview: 'Overview',
		metrics: 'Key metrics',
		records: 'Timeline',
		empty: 'There is no company information in this scope.',
		scope: 'Shows only what the categories you can read cover.',
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
			grant: 'Grant',
			product: 'Product',
			certification: 'Certification',
			ip: 'IP',
			award: 'Award',
			reference: 'Reference'
		}
	}
};
