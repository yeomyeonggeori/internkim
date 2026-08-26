import type { CRMOrganization, CRMActivity, CRMContact, CRMIntakeDraft, CRMNextAction, CRMOpportunity, CRMOpportunityStage, CRMReportSummary } from './crm-types';

const defaultCRMBusiness = '샘플거리';

export const crmOrganizations: CRMOrganization[] = [
	{
		id: 'organization-hanyang-startup',
		name: '예시 스타트업 지원단',
		types: ['partner', 'sponsor'],
		status: 'active',
		importance: 'high',
		ownerName: '김테스트04',
		ownerEmail: 'owner04@example.com',
		team: '운영',
		address: '서울특별시 성동구 왕십리로 222',
		tags: ['행사', '멘토링', '장기 파트너'],
		description: '상반기 행사 운영과 멘토링 프로그램을 같이 진행하는 핵심 파트너입니다.',
		lastContactDate: '2026-07-18',
		nextActionDate: '2026-07-23',
		openOpportunityCount: 2,
		expectedValues: { KRW: 18000000 }
	},
	{
		id: 'organization-seoul-impact',
		name: '샘플 임팩트 랩',
		types: ['customer', 'partner'],
		status: 'prospect',
		importance: 'high',
		ownerName: '김테스트09',
		ownerEmail: 'owner09@example.com',
		team: '세일즈',
		tags: ['도입 검토', '워크숍', '파일럿'],
		description: '운영 자동화와 출결 리포트 도입을 검토 중인 잠재 고객사입니다.',
		lastContactDate: '2026-07-16',
		nextActionDate: '2026-07-22',
		openOpportunityCount: 2,
		expectedValues: { KRW: 8500000, USD: 40000 }
	},
	{
		id: 'organization-blue-campus',
		name: '예시캠퍼스 재단',
		types: ['sponsor'],
		status: 'paused',
		importance: 'medium',
		ownerName: '김테스트18',
		ownerEmail: 'owner18@example.com',
		team: '파트너십',
		tags: ['후원', '계약 검토', '예산 심사'],
		description: '하반기 후원 패키지와 참여 학생 리포트 범위를 협의 중입니다.',
		lastContactDate: '2026-07-03',
		nextActionDate: '2026-07-29',
		openOpportunityCount: 2,
		expectedValues: { KRW: 15000000, JPY: 5500000 }
	},
	{
		id: 'organization-maker-house',
		name: '샘플메이커하우스',
		types: ['vendor'],
		status: 'active',
		importance: 'medium',
		ownerName: '김테스트23',
		ownerEmail: 'owner23@example.com',
		team: '운영',
		tags: ['공간', '장비', '현장 운영'],
		description: '행사 공간과 장비 협력사입니다. 응답 속도는 빠르지만 일정 확정이 늦어질 때가 있습니다.',
		lastContactDate: '2026-07-17',
		nextActionDate: '2026-07-24',
		openOpportunityCount: 2,
		expectedValues: { KRW: 4200000, EUR: 20000 }
	},
	{
		id: 'organization-ascend-ventures',
		name: '예시 벤처스',
		types: ['investor', 'partner'],
		status: 'prospect',
		importance: 'high',
		ownerName: '김테스트15',
		ownerEmail: 'owner15@example.com',
		team: '경영지원',
		tags: ['투자사', '월간 리포트', '후속 미팅'],
		description: '운영 지표와 행사 성과를 월간으로 공유하는 투자사입니다. 다음 라운드 자료 요청이 예정되어 있습니다.',
		lastContactDate: '2026-07-15',
		nextActionDate: '2026-07-26',
		openOpportunityCount: 1,
		expectedValues: { KRW: 50000000 }
	},
	{
		id: 'organization-campus-cloud',
		name: '샘플클라우드',
		types: ['customer'],
		status: 'active',
		importance: 'medium',
		ownerName: '김테스트09',
		ownerEmail: 'owner09@example.com',
		team: '세일즈',
		tags: ['SaaS', '관리자 도구', '분기 계약'],
		description: '동아리 운영팀을 대상으로 일정, 업무, 메일 연동 패키지를 테스트 중입니다.',
		lastContactDate: '2026-07-19',
		nextActionDate: '2026-07-25',
		openOpportunityCount: 1,
		expectedValues: { KRW: 6500000 }
	},
	{
		id: 'organization-green-catering',
		name: '예시케이터링',
		types: ['vendor'],
		status: 'active',
		importance: 'medium',
		ownerName: '김테스트23',
		ownerEmail: 'owner23@example.com',
		team: '운영',
		tags: ['케이터링', '단가표', '정산'],
		description: '행사 식음료 공급사입니다. 채식 옵션과 알레르기 표기를 꼼꼼히 확인해야 합니다.',
		lastContactDate: '2026-07-12',
		nextActionDate: '2026-07-23',
		openOpportunityCount: 1,
		expectedValues: { KRW: 3200000 }
	},
	{
		id: 'organization-root-finance',
		name: '샘플파이낸스',
		types: ['investor'],
		status: 'prospect',
		importance: 'high',
		ownerName: '김테스트15',
		ownerEmail: 'owner15@example.com',
		team: '경영지원',
		tags: ['투자 미팅', '데이터룸', '재무 지표'],
		description: '기관 투자 검토를 위해 월별 매출, 리텐션, 운영 비용 자료를 요청했습니다.',
		lastContactDate: '2026-07-10',
		nextActionDate: '2026-07-30',
		openOpportunityCount: 2,
		expectedValues: { KRW: 120000000, USD: 250000 }
	},
	{
		id: 'organization-design-lab-one',
		name: '예시디자인랩',
		types: ['partner'],
		status: 'paused',
		importance: 'low',
		ownerName: '김테스트04',
		ownerEmail: 'owner04@example.com',
		team: '운영',
		tags: ['브랜딩', '장기 미접촉'],
		description: '지난 행사 브랜딩을 맡았던 파트너입니다. 최근에는 신규 논의가 없어 재접촉 우선순위가 낮습니다.',
		lastContactDate: '2026-05-28',
		nextActionDate: '2026-08-05',
		openOpportunityCount: 1,
		expectedValues: { USD: 20000 }
	},
	{
		id: 'organization-youth-foundation',
		name: '샘플성장재단',
		types: ['sponsor', 'partner'],
		status: 'prospect',
		importance: 'high',
		ownerName: '김테스트18',
		ownerEmail: 'owner18@example.com',
		team: '파트너십',
		tags: ['후원', '성과 리포트', '공동 홍보'],
		description: '학생 성장 프로그램 후원과 공동 홍보 조건을 검토하고 있습니다.',
		lastContactDate: '2026-07-14',
		nextActionDate: '2026-07-24',
		openOpportunityCount: 1,
		expectedValues: { KRW: 22000000 }
	},
	{
		id: 'organization-k-edu-network',
		name: '예시에듀 네트워크',
		types: ['customer'],
		status: 'active',
		importance: 'medium',
		ownerName: '김테스트09',
		ownerEmail: 'owner09@example.com',
		team: '세일즈',
		tags: ['교육기관', '대시보드', '다중 조직'],
		description: '여러 교육 운영팀을 묶어 업무, 출결, 리포트 흐름을 통합하려는 고객사입니다.',
		lastContactDate: '2026-07-20',
		nextActionDate: '2026-07-27',
		openOpportunityCount: 2,
		expectedValues: { KRW: 18000000 }
	},
	{
		id: 'organization-motion-stage',
		name: '샘플스테이지',
		types: ['vendor'],
		status: 'paused',
		importance: 'medium',
		ownerName: '김테스트23',
		ownerEmail: 'owner23@example.com',
		team: '운영',
		tags: ['무대', '음향', '계약 보류'],
		description: '무대와 음향 장비 견적을 받았지만 행사 규모가 확정되지 않아 계약이 보류되어 있습니다.',
		lastContactDate: '2026-06-25',
		nextActionDate: '2026-07-31',
		openOpportunityCount: 1,
		expectedValues: { KRW: 7000000 }
	},
	{
		id: 'organization-future-alumni',
		name: '예시동문회',
		types: ['sponsor'],
		status: 'paused',
		importance: 'medium',
		ownerName: '김테스트18',
		ownerEmail: 'owner18@example.com',
		team: '파트너십',
		tags: ['성사', '장학 후원', '동문 네트워크'],
		description: '장학 후원과 멘토 추천이 확정되어 후속 운영만 남아 있습니다.',
		lastContactDate: '2026-07-21',
		nextActionDate: '2026-08-01',
		openOpportunityCount: 1,
		expectedValues: { KRW: 10000000 }
	},
	{
		id: 'organization-delta-impact',
		name: '샘플임팩트',
		types: ['investor'],
		status: 'paused',
		importance: 'medium',
		ownerName: '김테스트15',
		ownerEmail: 'owner15@example.com',
		team: '경영지원',
		tags: ['투자 보류', '리스크 검토'],
		description: '이번 라운드 참여는 보류되었지만 분기별 업데이트는 계속 받기로 했습니다.',
		lastContactDate: '2026-06-18',
		nextActionDate: '2026-09-02',
		openOpportunityCount: 0,
		expectedValues: {}
	},
	{
		id: 'organization-campus-media',
		name: '예시미디어',
		types: ['partner', 'vendor'],
		status: 'active',
		importance: 'low',
		ownerName: '김테스트04',
		ownerEmail: 'owner04@example.com',
		team: '운영',
		tags: ['홍보', '영상', '콘텐츠'],
		description: '행사 홍보 영상과 숏폼 콘텐츠 제작을 담당하는 파트너입니다.',
		lastContactDate: '2026-07-13',
		nextActionDate: '2026-07-28',
		openOpportunityCount: 1,
		expectedValues: { KRW: 3000000, JPY: 3200000 }
	},
	{
		id: 'organization-ai-learning',
		name: '샘플러닝랩',
		types: ['customer'],
		status: 'prospect',
		importance: 'high',
		ownerName: '김테스트09',
		ownerEmail: 'owner09@example.com',
		team: '세일즈',
		tags: ['기업 교육', '메일 연동', '파일 자동 입력'],
		description: '영업 교육 운영팀에서 메일, 파일, 회의록 기반 입력 자동화를 강하게 원하고 있습니다.',
		lastContactDate: '2026-07-11',
		nextActionDate: '2026-07-25',
		openOpportunityCount: 2,
		expectedValues: { KRW: 28000000, EUR: 50000 }
	},
	{
		id: 'organization-volunteer-union',
		name: '예시봉사연합',
		types: ['partner'],
		status: 'active',
		importance: 'low',
		ownerName: '김테스트23',
		ownerEmail: 'owner23@example.com',
		team: '운영',
		tags: ['봉사자', '운영 협력', '일정 공유'],
		description: '현장 봉사자 모집과 사전 교육 일정을 같이 조율하는 운영 파트너입니다.',
		lastContactDate: '2026-07-09',
		nextActionDate: '2026-07-26',
		openOpportunityCount: 1,
		expectedValues: { KRW: 7500000 }
	},
	{
		id: 'organization-blue-river-hotel',
		name: '샘플호텔',
		types: ['vendor'],
		status: 'prospect',
		importance: 'medium',
		ownerName: '김테스트23',
		ownerEmail: 'owner23@example.com',
		team: '운영',
		tags: ['숙박', '견적', '단체 예약'],
		description: '외부 연사 숙박과 조식 포함 단체 예약 조건을 협의 중입니다.',
		lastContactDate: '2026-07-08',
		nextActionDate: '2026-07-24',
		openOpportunityCount: 2,
		expectedValues: { KRW: 9000000, JPY: 8000000 }
	},
	{
		id: 'organization-social-innovation-fund',
		name: '예시이노베이션 펀드',
		types: ['investor', 'sponsor'],
		status: 'active',
		importance: 'high',
		ownerName: '김테스트15',
		ownerEmail: 'owner15@example.com',
		team: '경영지원',
		tags: ['임팩트 투자', '후원 가능성', '성과 지표'],
		description: '임팩트 지표와 후원 전환 데이터를 함께 보는 전략 투자 후보입니다.',
		lastContactDate: '2026-07-06',
		nextActionDate: '2026-07-29',
		openOpportunityCount: 1,
		expectedValues: { KRW: 70000000 }
	},
	{
		id: 'organization-city-office',
		name: '예시구청 청년정책과',
		types: ['customer', 'partner'],
		status: 'paused',
		importance: 'high',
		ownerName: '김테스트04',
		ownerEmail: 'owner04@example.com',
		team: '운영',
		tags: ['공공기관', '제안서', '일정 지연'],
		description: '청년 행사 운영 시스템 도입을 검토 중이지만 내부 심의 일정 때문에 의사결정이 늦어지고 있습니다.',
		lastContactDate: '2026-06-29',
		nextActionDate: '2026-08-04',
		openOpportunityCount: 1,
		expectedValues: { KRW: 35000000 }
	}
];

export const crmContacts: CRMContact[] = [
	{
		id: 'contact-hanyang-leesample',
		organizationID: 'organization-hanyang-startup',
		name: '김테스트17',
		title: '프로그램 매니저',
		email: 'contact01@example.com',
		phone: '000-0000-0101',
		note: '행사 운영 일정과 멘토 매칭을 함께 조율합니다.'
	},
	{
		id: 'contact-seoul-impact-choi',
		organizationID: 'organization-seoul-impact',
		name: '김테스트25',
		title: '운영 리드',
		email: 'contact02@example.com',
		phone: '000-0000-0102',
		note: '데모 이후 비용 범위와 도입 일정에 민감합니다.'
	},
	{
		id: 'contact-blue-campus-kang',
		organizationID: 'organization-blue-campus',
		name: '김테스트01',
		title: '파트너십 디렉터',
		email: 'contact03@example.com',
		phone: '000-0000-0103',
	},
	{
		id: 'contact-maker-house-yoon',
		organizationID: 'organization-maker-house',
		name: '김테스트16',
		title: '공간 운영 매니저',
		email: 'contact04@example.com',
		phone: '000-0000-0104',
	},
	{
		id: 'contact-ascend-kim',
		organizationID: 'organization-ascend-ventures',
		name: '김테스트03',
		title: '파트너',
		email: 'contact05@example.com',
		phone: '000-0000-0105',
		note: '월간 운영 지표와 다음 분기 후원 계획을 함께 확인합니다.'
	},
	{
		id: 'contact-campus-cloud-han',
		organizationID: 'organization-campus-cloud',
		name: '김테스트27',
		title: '제품 운영 매니저',
		email: 'contact06@example.com',
		phone: '000-0000-0106',
	},
	{
		id: 'contact-green-catering-jang',
		organizationID: 'organization-green-catering',
		name: '김테스트21',
		title: '영업 담당',
		email: 'contact07@example.com',
		phone: '000-0000-0107',
		note: '알레르기 표기 파일은 행사 5일 전까지 받아야 합니다.'
	},
	{
		id: 'contact-root-finance-moon',
		organizationID: 'organization-root-finance',
		name: '김테스트07',
		title: '투자심사역',
		email: 'contact08@example.com',
		phone: '000-0000-0108',
	},
	{
		id: 'contact-design-lab-one-rha',
		organizationID: 'organization-design-lab-one',
		name: '김테스트06',
		title: '크리에이티브 디렉터',
		email: 'contact09@example.com',
	},
	{
		id: 'contact-youth-foundation-song',
		organizationID: 'organization-youth-foundation',
		name: '김테스트11',
		title: '사업기획팀장',
		email: 'contact10@example.com',
		phone: '000-0000-0110',
	},
	{
		id: 'contact-k-edu-network-kwon',
		organizationID: 'organization-k-edu-network',
		name: '김테스트02',
		title: 'DX 추진팀장',
		email: 'contact11@example.com',
		phone: '000-0000-0111',
	},
	{
		id: 'contact-motion-stage-oh',
		organizationID: 'organization-motion-stage',
		name: '김테스트14',
		title: '프로덕션 매니저',
		email: 'contact12@example.com',
	},
	{
		id: 'contact-future-alumni-yang',
		organizationID: 'organization-future-alumni',
		name: '김테스트13',
		title: '사무국장',
		email: 'contact13@example.com',
		phone: '000-0000-0113',
	},
	{
		id: 'contact-delta-impact-park',
		organizationID: 'organization-delta-impact',
		name: '김테스트08',
		title: '어소시에이트',
		email: 'contact14@example.com',
	},
	{
		id: 'contact-campus-media-lim',
		organizationID: 'organization-campus-media',
		name: '김테스트20',
		title: '콘텐츠 PD',
		email: 'contact15@example.com',
		phone: '000-0000-0115',
	},
	{
		id: 'contact-ai-learning-shin',
		organizationID: 'organization-ai-learning',
		name: '김테스트12',
		title: 'B2B 사업개발 리드',
		email: 'contact16@example.com',
		phone: '000-0000-0116',
		note: '파일 업로드 후 자동 분류 정확도를 가장 중요하게 봅니다.'
	},
	{
		id: 'contact-volunteer-union-jeon',
		organizationID: 'organization-volunteer-union',
		name: '김테스트22',
		title: '교육 코디네이터',
		email: 'contact17@example.com',
	},
	{
		id: 'contact-blue-river-hotel-baek',
		organizationID: 'organization-blue-river-hotel',
		name: '김테스트10',
		title: '세일즈 매니저',
		email: 'contact18@example.com',
		phone: '000-0000-0118',
	},
	{
		id: 'contact-social-innovation-fund-cho',
		organizationID: 'organization-social-innovation-fund',
		name: '김테스트24',
		title: '임팩트 파트너',
		email: 'contact19@example.com',
		phone: '000-0000-0119',
	},
	{
		id: 'contact-city-office-hong',
		organizationID: 'organization-city-office',
		name: '김테스트28',
		title: '주무관',
		email: 'contact20@example.com',
		phone: '000-0000-0120',
	}
];

const crmOpportunityFixtures: Array<Omit<CRMOpportunity, 'business'>> = [
	{
		id: 'opportunity-hanyang-mentoring',
		organizationID: 'organization-hanyang-startup',
		name: '하반기 멘토링 운영 계약',
		stage: 'review',
		currency: 'KRW',
		kind: 'partnership',
		ownerName: '김테스트04',
		expectedValue: 12000000,
		importance: 'high',
		nextActionID: 'action-hanyang-contract',
		targetDate: '2026-08-05',
		staleDays: 4
	},
	{
		id: 'opportunity-hanyang-demo-day',
		organizationID: 'organization-hanyang-startup',
		name: '데모데이 운영 후원',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'sponsorship',
		ownerName: '김테스트04',
		expectedValue: 6000000,
		importance: 'medium',
		nextActionID: 'action-hanyang-demo-day',
		targetDate: '2026-08-18',
		staleDays: 2
	},
	{
		id: 'opportunity-design-lab-global',
		organizationID: 'organization-design-lab-one',
		name: '글로벌 브랜드 디자인 리뉴얼',
		stage: 'review',
		currency: 'USD',
		kind: 'procurement',
		ownerName: '김테스트04',
		expectedValue: 20000,
		importance: 'medium',
		nextActionID: 'action-design-global-proposal',
		targetDate: '2026-08-28',
		staleDays: 5,
		description: '영문 브랜드 가이드와 글로벌 행사 템플릿 제작 범위를 협의 중입니다.'
	},
	{
		id: 'opportunity-campus-media-japan',
		organizationID: 'organization-campus-media',
		name: '일본 캠퍼스 콘텐츠 배급',
		stage: 'done',
		currency: 'JPY',
		kind: 'partnership',
		ownerName: '김테스트04',
		expectedValue: 3200000,
		importance: 'low',
		targetDate: '2026-07-18',
		staleDays: 0,
		description: '일본 대학 채널에 행사 인터뷰 콘텐츠를 배급하는 협업이 확정되었습니다.'
	},
	{
		id: 'opportunity-ai-learning-europe',
		organizationID: 'organization-ai-learning',
		name: '유럽 교육기관 라이선스',
		stage: 'in_progress',
		currency: 'EUR',
		kind: 'sales',
		ownerName: '김테스트09',
		expectedValue: 50000,
		importance: 'high',
		nextActionID: 'action-ai-learning-europe-license',
		targetDate: '2026-09-12',
		staleDays: 7,
		description: '유럽 교육기관 3곳을 대상으로 다국어 운영 라이선스를 제안했습니다.'
	},
	{
		id: 'opportunity-seoul-impact-pilot',
		organizationID: 'organization-seoul-impact',
		name: '운영 자동화 파일럿',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'sales',
		ownerName: '김테스트09',
		expectedValue: 8500000,
		importance: 'high',
		nextActionID: 'action-seoul-impact-demo',
		targetDate: '2026-08-02',
		staleDays: 7
	},
	{
		id: 'opportunity-blue-campus-sponsor',
		organizationID: 'organization-blue-campus',
		name: '하반기 후원 패키지',
		stage: 'on_hold',
		currency: 'KRW',
		kind: 'sponsorship',
		ownerName: '김테스트18',
		expectedValue: 15000000,
		importance: 'medium',
		nextActionID: 'action-blue-campus-renew',
		targetDate: '2026-08-12',
		staleDays: 19
	},
	{
		id: 'opportunity-maker-house-space',
		organizationID: 'organization-maker-house',
		name: '8월 행사 공간 예약',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'procurement',
		ownerName: '김테스트23',
		expectedValue: 4200000,
		importance: 'medium',
		nextActionID: 'action-maker-house-hold',
		targetDate: '2026-07-30',
		staleDays: 3
	},
	{
		id: 'opportunity-ascend-followup',
		organizationID: 'organization-ascend-ventures',
		name: '하반기 투자자 업데이트',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'investment',
		ownerName: '김테스트15',
		expectedValue: 50000000,
		importance: 'high',
		nextActionID: 'action-ascend-report',
		targetDate: '2026-08-07',
		staleDays: 6
	},
	{
		id: 'opportunity-campus-cloud-quarter',
		organizationID: 'organization-campus-cloud',
		name: '분기 운영 패키지',
		stage: 'review',
		currency: 'KRW',
		kind: 'sales',
		ownerName: '김테스트09',
		expectedValue: 6500000,
		importance: 'medium',
		nextActionID: 'action-campus-cloud-security',
		targetDate: '2026-08-09',
		staleDays: 1
	},
	{
		id: 'opportunity-green-catering-order',
		organizationID: 'organization-green-catering',
		name: '8월 워크숍 케이터링',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'procurement',
		ownerName: '김테스트23',
		expectedValue: 3200000,
		importance: 'medium',
		nextActionID: 'action-green-catering-menu',
		targetDate: '2026-07-29',
		staleDays: 5
	},
	{
		id: 'opportunity-root-finance-round',
		organizationID: 'organization-root-finance',
		name: '시드 브릿지 투자 검토',
		stage: 'waiting',
		currency: 'KRW',
		kind: 'investment',
		ownerName: '김테스트15',
		expectedValue: 120000000,
		importance: 'high',
		nextActionID: 'action-root-finance-data-room',
		targetDate: '2026-09-15',
		staleDays: 11
	},
	{
		id: 'opportunity-youth-foundation-sponsor',
		organizationID: 'organization-youth-foundation',
		name: '학생 성장 프로그램 후원',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'sponsorship',
		ownerName: '김테스트18',
		expectedValue: 22000000,
		importance: 'high',
		nextActionID: 'action-youth-foundation-report',
		targetDate: '2026-08-20',
		staleDays: 4
	},
	{
		id: 'opportunity-k-edu-dashboard',
		organizationID: 'organization-k-edu-network',
		name: '운영 대시보드 도입',
		stage: 'review',
		currency: 'KRW',
		kind: 'sales',
		ownerName: '김테스트09',
		expectedValue: 12000000,
		importance: 'medium',
		nextActionID: 'action-k-edu-procurement',
		targetDate: '2026-08-16',
		staleDays: 2
	},
	{
		id: 'opportunity-k-edu-training',
		organizationID: 'organization-k-edu-network',
		name: '관리자 교육 패키지',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'sales',
		ownerName: '김테스트09',
		expectedValue: 6000000,
		importance: 'low',
		nextActionID: 'action-k-edu-training',
		targetDate: '2026-08-25',
		staleDays: 1
	},
	{
		id: 'opportunity-motion-stage-equipment',
		organizationID: 'organization-motion-stage',
		name: '무대 음향 장비 계약',
		stage: 'on_hold',
		currency: 'KRW',
		kind: 'procurement',
		ownerName: '김테스트23',
		expectedValue: 7000000,
		importance: 'medium',
		nextActionID: 'action-motion-stage-estimate',
		targetDate: '2026-08-03',
		staleDays: 14
	},
	{
		id: 'opportunity-future-alumni-scholarship',
		organizationID: 'organization-future-alumni',
		name: '장학 후원 확정',
		stage: 'done',
		currency: 'KRW',
		kind: 'sponsorship',
		ownerName: '김테스트18',
		expectedValue: 10000000,
		importance: 'medium',
		nextActionID: 'action-future-alumni-thanks',
		targetDate: '2026-07-21',
		staleDays: 0
	},
	{
		id: 'opportunity-delta-impact-round',
		organizationID: 'organization-delta-impact',
		name: '전략 투자 검토',
		stage: 'lost',
		lostReason: '이사회가 예산 우선순위를 다른 사업부로 옮기며 무산',
		currency: 'KRW',
		kind: 'investment',
		ownerName: '김테스트15',
		importance: 'medium',
		nextActionID: 'action-delta-impact-update',
		targetDate: '2026-07-02',
		staleDays: 0
	},
	{
		id: 'opportunity-campus-media-video',
		organizationID: 'organization-campus-media',
		name: '행사 홍보 영상 제작',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'partnership',
		ownerName: '김테스트04',
		expectedValue: 3000000,
		importance: 'low',
		nextActionID: 'action-campus-media-brief',
		targetDate: '2026-08-01',
		staleDays: 3
	},
	{
		id: 'opportunity-ai-learning-enterprise',
		organizationID: 'organization-ai-learning',
		name: '기업 교육 CRM 파일럿',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'sales',
		ownerName: '김테스트09',
		expectedValue: 28000000,
		importance: 'high',
		nextActionID: 'action-ai-learning-sample',
		targetDate: '2026-08-22',
		staleDays: 8
	},
	{
		id: 'opportunity-blue-river-room',
		organizationID: 'organization-blue-river-hotel',
		name: '외부 연사 숙박 예약',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'procurement',
		ownerName: '김테스트23',
		expectedValue: 9000000,
		importance: 'medium',
		nextActionID: 'action-blue-river-contract',
		targetDate: '2026-08-06',
		staleDays: 6
	},
	{
		id: 'opportunity-social-innovation-fund',
		organizationID: 'organization-social-innovation-fund',
		name: '임팩트 투자 및 후원 검토',
		stage: 'review',
		currency: 'KRW',
		kind: 'investment',
		ownerName: '김테스트15',
		expectedValue: 70000000,
		importance: 'high',
		nextActionID: 'action-social-innovation-impact',
		targetDate: '2026-09-01',
		staleDays: 9
	},
	{
		id: 'opportunity-city-office-program',
		organizationID: 'organization-city-office',
		name: '청년 행사 운영 시스템',
		stage: 'waiting',
		currency: 'KRW',
		kind: 'partnership',
		ownerName: '김테스트04',
		expectedValue: 35000000,
		importance: 'high',
		nextActionID: 'action-city-office-review',
		targetDate: '2026-09-05',
		staleDays: 16
	},
	{
		id: 'opportunity-volunteer-operation',
		organizationID: 'organization-volunteer-union',
		name: '지역 봉사단 운영 협약',
		stage: 'in_progress',
		currency: 'KRW',
		kind: 'partnership',
		ownerName: '김테스트23',
		expectedValue: 7500000,
		importance: 'low',
		nextActionID: 'action-volunteer-operation-scope',
		targetDate: '2026-08-30',
		staleDays: 1,
		description: '행사 안내와 참가자 동선 지원을 위한 공동 운영 범위를 조율합니다.'
	},
	{
		id: 'opportunity-seoul-impact-asia',
		organizationID: 'organization-seoul-impact',
		name: '동남아 운영 자동화 확장',
		stage: 'in_progress',
		currency: 'USD',
		kind: 'sales',
		ownerName: '김테스트09',
		expectedValue: 40000,
		importance: 'high',
		targetDate: '2026-09-20',
		staleDays: 2,
		description: '동남아 파트너 프로그램으로 운영 자동화 적용 범위를 확장하는 안입니다.'
	},
	{
		id: 'opportunity-blue-campus-japan',
		organizationID: 'organization-blue-campus',
		name: '일본 교류 프로그램 후원',
		stage: 'on_hold',
		currency: 'JPY',
		kind: 'sponsorship',
		ownerName: '김테스트18',
		expectedValue: 5500000,
		importance: 'medium',
		targetDate: '2026-10-02',
		staleDays: 24,
		description: '환율과 현지 행사 일정이 확정될 때까지 후원 검토가 보류된 상태입니다.'
	},
	{
		id: 'opportunity-maker-house-global',
		organizationID: 'organization-maker-house',
		name: '글로벌 메이커톤 장비 패키지',
		stage: 'in_progress',
		currency: 'EUR',
		kind: 'procurement',
		ownerName: '김테스트23',
		expectedValue: 20000,
		importance: 'medium',
		targetDate: '2026-09-08',
		staleDays: 4,
		description: '해외 참가팀용 제작 장비와 안전 교육을 포함한 패키지 견적입니다.',
		calendarRegistrationState: 'failed'
	},
	{
		id: 'opportunity-root-finance-overseas',
		organizationID: 'organization-root-finance',
		name: '해외 투자자 공동 라운드',
		stage: 'waiting',
		currency: 'USD',
		kind: 'investment',
		ownerName: '김테스트15',
		expectedValue: 250000,
		importance: 'high',
		nextActionID: 'action-root-finance-overseas-round',
		targetDate: '2026-10-15',
		staleDays: 12,
		description: '해외 임팩트 펀드와 공동 투자 가능성을 검토하는 초기 단계입니다.'
	},
	{
		id: 'opportunity-blue-river-japan',
		organizationID: 'organization-blue-river-hotel',
		name: '일본 참가자 숙박 블록',
		stage: 'review',
		currency: 'JPY',
		kind: 'procurement',
		ownerName: '김테스트23',
		expectedValue: 8000000,
		importance: 'medium',
		nextActionID: 'action-blue-river-japan-room',
		targetDate: '2026-09-03',
		staleDays: 3,
		description: '일본 참가자 40명의 객실과 조식 조건을 묶어 협상 중입니다.'
	},
	{
		id: 'opportunity-future-alumni-europe',
		organizationID: 'organization-future-alumni',
		name: '유럽 동문 네트워크 후원',
		stage: 'waiting',
		currency: 'EUR',
		kind: 'sponsorship',
		ownerName: '김테스트18',
		importance: 'low',
		targetDate: '2026-10-30',
		staleDays: 0,
		description: '예산 범위를 확인하기 전인 초기 후원 논의로 금액은 아직 입력하지 않았습니다.'
	}
];

export const crmOpportunities: CRMOpportunity[] = crmOpportunityFixtures.map((opportunity) => ({
	...opportunity,
	business: defaultCRMBusiness
}));

export const crmNextActions: CRMNextAction[] = [
	{
		id: 'action-hanyang-contract',
		organizationID: 'organization-hanyang-startup',
		opportunityID: 'opportunity-hanyang-mentoring',
		title: '계약서 수정본 확인 요청',
		ownerName: '김테스트04',
		dueDate: '2026-07-23',
		status: 'in_progress'
	},
	{
		id: 'action-hanyang-demo-day',
		organizationID: 'organization-hanyang-startup',
		opportunityID: 'opportunity-hanyang-demo-day',
		title: '데모데이 후원 노출 범위 회신',
		ownerName: '김테스트04',
		dueDate: '2026-07-28',
		status: 'waiting'
	},
	{
		id: 'action-seoul-impact-demo',
		organizationID: 'organization-seoul-impact',
		opportunityID: 'opportunity-seoul-impact-pilot',
		title: '출결 리포트 데모 일정 확정',
		ownerName: '김테스트09',
		dueDate: '2026-07-22',
		status: 'in_progress'
	},
	{
		id: 'action-blue-campus-renew',
		organizationID: 'organization-blue-campus',
		opportunityID: 'opportunity-blue-campus-sponsor',
		title: '후원 범위 축소안 공유',
		ownerName: '김테스트18',
		dueDate: '2026-07-29',
		status: 'waiting'
	},
	{
		id: 'action-maker-house-hold',
		organizationID: 'organization-maker-house',
		opportunityID: 'opportunity-maker-house-space',
		title: '8월 2주차 공간 홀드 가능 여부 확인',
		ownerName: '김테스트23',
		dueDate: '2026-07-24',
		status: 'waiting'
	},
	{
		id: 'action-ascend-report',
		organizationID: 'organization-ascend-ventures',
		opportunityID: 'opportunity-ascend-followup',
		title: '6월 운영 지표와 행사 성과 파일 공유',
		ownerName: '김테스트15',
		dueDate: '2026-07-26',
		status: 'todo'
	},
	{
		id: 'action-campus-cloud-security',
		organizationID: 'organization-campus-cloud',
		opportunityID: 'opportunity-campus-cloud-quarter',
		title: '보안 체크리스트 답변 정리',
		ownerName: '김테스트09',
		dueDate: '2026-07-25',
		status: 'in_progress'
	},
	{
		id: 'action-green-catering-menu',
		organizationID: 'organization-green-catering',
		opportunityID: 'opportunity-green-catering-order',
		title: '채식 옵션과 알레르기 표기 파일 요청',
		ownerName: '김테스트23',
		dueDate: '2026-07-23',
		status: 'waiting'
	},
	{
		id: 'action-root-finance-data-room',
		organizationID: 'organization-root-finance',
		opportunityID: 'opportunity-root-finance-round',
		title: '데이터룸 권한과 월간 지표 업로드',
		ownerName: '김테스트15',
		dueDate: '2026-07-30',
		status: 'todo'
	},
	{
		id: 'action-design-lab-reconnect',
		organizationID: 'organization-design-lab-one',
		title: '하반기 브랜딩 가능 일정 확인',
		ownerName: '김테스트04',
		dueDate: '2026-08-05',
		status: 'todo'
	},
	{
		id: 'action-youth-foundation-report',
		organizationID: 'organization-youth-foundation',
		opportunityID: 'opportunity-youth-foundation-sponsor',
		title: '성과 리포트 샘플과 홍보 문구 공유',
		ownerName: '김테스트18',
		dueDate: '2026-07-24',
		status: 'in_progress'
	},
	{
		id: 'action-k-edu-procurement',
		organizationID: 'organization-k-edu-network',
		opportunityID: 'opportunity-k-edu-dashboard',
		title: '구매 절차와 보안 검토 담당자 확인',
		ownerName: '김테스트09',
		dueDate: '2026-07-27',
		status: 'waiting'
	},
	{
		id: 'action-k-edu-training',
		organizationID: 'organization-k-edu-network',
		opportunityID: 'opportunity-k-edu-training',
		title: '관리자 교육 범위 산정',
		ownerName: '김테스트09',
		dueDate: '2026-07-31',
		status: 'todo'
	},
	{
		id: 'action-motion-stage-estimate',
		organizationID: 'organization-motion-stage',
		opportunityID: 'opportunity-motion-stage-equipment',
		title: '행사 규모별 견적 2안 요청',
		ownerName: '김테스트23',
		dueDate: '2026-07-18',
		status: 'todo'
	},
	{
		id: 'action-future-alumni-thanks',
		organizationID: 'organization-future-alumni',
		opportunityID: 'opportunity-future-alumni-scholarship',
		title: '후원 확정 감사 메일 발송',
		ownerName: '김테스트18',
		dueDate: '2026-07-21',
		status: 'done'
	},
	{
		id: 'action-delta-impact-update',
		organizationID: 'organization-delta-impact',
		opportunityID: 'opportunity-delta-impact-round',
		title: '분기 업데이트 수신 목록 유지 확인',
		ownerName: '김테스트15',
		dueDate: '2026-08-10',
		status: 'todo'
	},
	{
		id: 'action-campus-media-brief',
		organizationID: 'organization-campus-media',
		opportunityID: 'opportunity-campus-media-video',
		title: '촬영 브리프와 레퍼런스 영상 전달',
		ownerName: '김테스트04',
		dueDate: '2026-07-28',
		status: 'in_progress'
	},
	{
		id: 'action-ai-learning-sample',
		organizationID: 'organization-ai-learning',
		opportunityID: 'opportunity-ai-learning-enterprise',
		title: '샘플 파일 30건 자동 입력 결과 공유',
		ownerName: '김테스트09',
		dueDate: '2026-07-25',
		status: 'in_progress'
	},
	{
		id: 'action-volunteer-union-training',
		organizationID: 'organization-volunteer-union',
		title: '봉사자 사전 교육 일정표 확인',
		ownerName: '김테스트23',
		dueDate: '2026-07-26',
		status: 'waiting'
	},
	{
		id: 'action-blue-river-contract',
		organizationID: 'organization-blue-river-hotel',
		opportunityID: 'opportunity-blue-river-room',
		title: '객실 블록 계약서와 취소 조건 확인',
		ownerName: '김테스트23',
		dueDate: '2026-07-24',
		status: 'waiting'
	},
	{
		id: 'action-social-innovation-impact',
		organizationID: 'organization-social-innovation-fund',
		opportunityID: 'opportunity-social-innovation-fund',
		title: '임팩트 지표 산식과 데이터 출처 설명',
		ownerName: '김테스트15',
		dueDate: '2026-07-29',
		status: 'in_progress'
	},
	{
		id: 'action-city-office-review',
		organizationID: 'organization-city-office',
		opportunityID: 'opportunity-city-office-program',
		title: '내부 심의 일정과 제안서 수정 범위 확인',
		ownerName: '김테스트04',
		dueDate: '2026-08-04',
		status: 'todo'
	},
	{
		id: 'action-design-global-proposal',
		organizationID: 'organization-design-lab-one',
		opportunityID: 'opportunity-design-lab-global',
		title: '영문 브랜드 가이드 수정 견적 확인',
		ownerName: '김테스트04',
		dueDate: '2026-08-01',
		status: 'todo'
	},
	{
		id: 'action-volunteer-operation-scope',
		organizationID: 'organization-volunteer-union',
		opportunityID: 'opportunity-volunteer-operation',
		title: '봉사자 역할과 현장 책임 범위 합의',
		ownerName: '김테스트23',
		dueDate: '2026-08-04',
		status: 'waiting'
	},
	{
		id: 'action-ai-learning-europe-license',
		organizationID: 'organization-ai-learning',
		opportunityID: 'opportunity-ai-learning-europe',
		title: '유럽 기관별 라이선스 조건 회신',
		ownerName: '김테스트09',
		dueDate: '2026-08-08',
		status: 'in_progress'
	},
	{
		id: 'action-root-finance-overseas-round',
		organizationID: 'organization-root-finance',
		opportunityID: 'opportunity-root-finance-overseas',
		title: '해외 투자자용 영문 데이터룸 준비',
		ownerName: '김테스트15',
		dueDate: '2026-08-15',
		status: 'todo'
	},
	{
		id: 'action-blue-river-japan-room',
		organizationID: 'organization-blue-river-hotel',
		opportunityID: 'opportunity-blue-river-japan',
		title: '일본 참가자 객실 배정표 전달',
		ownerName: '김테스트23',
		dueDate: '2026-08-03',
		status: 'waiting'
	}
];

const crmActivityFixtures: Array<Omit<CRMActivity, 'business' | 'opportunityID'>> = [
	{
		id: 'activity-future-alumni-won',
		taskID: 'crm-task-future-alumni-won',
		organizationID: 'organization-future-alumni',
		kind: 'stage_change',
		title: '장학 후원 성사 처리',
		occurredAt: '2026-07-21T16:40:00+09:00',
		summary: '장학 후원 1,000만 원과 멘토 4명 추천이 확정되었습니다.'
	},
	{
		id: 'activity-k-edu-meeting',
		taskID: 'crm-task-k-edu-meeting',
		organizationID: 'organization-k-edu-network',
		kind: 'meeting',
		title: '다중 조직 권한 구조 협의',
		occurredAt: '2026-07-20T13:20:00+09:00',
		summary: '교육기관별 관리자 권한과 리포트 공개 범위를 분리해야 한다는 요구가 확인되었습니다.'
	},
	{
		id: 'activity-campus-cloud-call',
		taskID: 'crm-task-campus-cloud-call',
		organizationID: 'organization-campus-cloud',
		kind: 'call',
		title: '보안 체크리스트 수신',
		occurredAt: '2026-07-19T10:10:00+09:00',
		summary: 'SSO, 데이터 보관 기간, 관리자 감사 로그 항목을 포함한 체크리스트를 받았습니다.'
	},
	{
		id: 'activity-hanyang-meeting',
		taskID: 'crm-task-hanyang-meeting',
		organizationID: 'organization-hanyang-startup',
		kind: 'meeting',
		title: '멘토링 운영 범위 협의',
		occurredAt: '2026-07-18T14:30:00+09:00',
		summary: '멘토 12명 기준 운영안으로 좁히고 계약서 수정본을 보내기로 했습니다.'
	},
	{
		id: 'activity-maker-call',
		taskID: 'crm-task-maker-call',
		organizationID: 'organization-maker-house',
		kind: 'call',
		title: '행사 공간 가능 일정 확인',
		occurredAt: '2026-07-17T11:00:00+09:00',
		summary: '8월 2주차 평일 저녁은 가능하지만 주말은 대기 상태입니다.'
	},
	{
		id: 'activity-seoul-email',
		taskID: 'crm-task-seoul-email',
		organizationID: 'organization-seoul-impact',
		kind: 'email',
		title: '파일럿 제안서 발송',
		occurredAt: '2026-07-16T10:15:00+09:00',
		summary: '출결, 일정, 업무 리포트를 묶은 4주 파일럿 제안서를 보냈습니다.'
	},
	{
		id: 'activity-ascend-file',
		taskID: 'crm-task-ascend-file',
		organizationID: 'organization-ascend-ventures',
		kind: 'file',
		title: '투자자 업데이트 초안 업로드',
		occurredAt: '2026-07-15T17:40:00+09:00',
		summary: '6월 활동 수, 참여자 유지율, 후원 전환율을 포함한 초안 파일을 정리했습니다.'
	},
	{
		id: 'activity-youth-foundation-note',
		taskID: 'crm-task-youth-foundation-note',
		organizationID: 'organization-youth-foundation',
		kind: 'note',
		title: '공동 홍보 문구 수정 요청',
		occurredAt: '2026-07-14T15:35:00+09:00',
		summary: '후원사 로고 노출 위치와 학생 인터뷰 문구를 조금 더 보수적으로 조정하기로 했습니다.'
	},
	{
		id: 'activity-campus-media-file',
		taskID: 'crm-task-campus-media-file',
		organizationID: 'organization-campus-media',
		kind: 'file',
		title: '촬영 구성안 업로드',
		occurredAt: '2026-07-13T18:05:00+09:00',
		summary: '행사 전후 인터뷰 컷과 현장 스케치 컷을 나누는 촬영 구성안이 업로드되었습니다.'
	},
	{
		id: 'activity-green-catering-email',
		taskID: 'crm-task-green-catering-email',
		organizationID: 'organization-green-catering',
		kind: 'email',
		title: '단가표와 메뉴 후보 수신',
		occurredAt: '2026-07-12T09:30:00+09:00',
		summary: '기본 도시락, 핑거푸드, 음료 패키지 단가표와 최소 주문 수량 조건을 받았습니다.'
	},
	{
		id: 'activity-ai-learning-demo',
		taskID: 'crm-task-ai-learning-demo',
		organizationID: 'organization-ai-learning',
		kind: 'meeting',
		title: '입력 자동화 데모 진행',
		occurredAt: '2026-07-11T16:10:00+09:00',
		summary: '메일과 첨부파일에서 관계처, 금액, 다음 액션을 추출하는 흐름을 시연했습니다.'
	},
	{
		id: 'activity-root-finance-request',
		taskID: 'crm-task-root-finance-request',
		organizationID: 'organization-root-finance',
		kind: 'email',
		title: '데이터룸 권한 요청',
		occurredAt: '2026-07-10T12:50:00+09:00',
		summary: '투자심사역 2명에게 데이터룸 권한을 열어달라는 요청을 받았습니다.'
	},
	{
		id: 'activity-volunteer-union-task',
		taskID: 'crm-task-volunteer-union-task',
		organizationID: 'organization-volunteer-union',
		kind: 'task',
		title: '봉사자 교육 일정 조율',
		occurredAt: '2026-07-09T14:00:00+09:00',
		summary: '봉사자 사전 교육을 온라인 1회와 현장 리허설 1회로 나누는 안을 공유했습니다.'
	},
	{
		id: 'activity-blue-river-call',
		taskID: 'crm-task-blue-river-call',
		organizationID: 'organization-blue-river-hotel',
		kind: 'call',
		title: '객실 블록 가능 여부 확인',
		occurredAt: '2026-07-08T11:25:00+09:00',
		summary: '트윈룸 12실과 조식 포함 조건은 가능하지만 취소 기한이 짧다는 점을 확인했습니다.'
	},
	{
		id: 'activity-social-innovation-meeting',
		taskID: 'crm-task-social-innovation-meeting',
		organizationID: 'organization-social-innovation-fund',
		kind: 'meeting',
		title: '임팩트 지표 검토 미팅',
		occurredAt: '2026-07-06T15:00:00+09:00',
		summary: '참여자 유지율, 후원 전환율, 멘토 참여 시간을 핵심 지표 후보로 정리했습니다.'
	},
	{
		id: 'activity-blue-note',
		taskID: 'crm-task-blue-note',
		organizationID: 'organization-blue-campus',
		kind: 'note',
		title: '후원 검토 일시 보류',
		occurredAt: '2026-07-03T16:20:00+09:00',
		summary: '예산 심사 일정 때문에 7월 말까지 결정이 밀릴 가능성이 있습니다.'
	},
	{
		id: 'activity-city-office-meeting',
		taskID: 'crm-task-city-office-meeting',
		organizationID: 'organization-city-office',
		kind: 'meeting',
		title: '공공기관 제안 범위 협의',
		occurredAt: '2026-06-29T10:30:00+09:00',
		summary: '청년정책과 내부 심의 후 제안서 양식과 개인정보 처리 항목을 다시 확인하기로 했습니다.'
	},
	{
		id: 'activity-motion-stage-note',
		taskID: 'crm-task-motion-stage-note',
		organizationID: 'organization-motion-stage',
		kind: 'note',
		title: '장비 견적 보류',
		occurredAt: '2026-06-25T13:45:00+09:00',
		summary: '참석자 규모가 확정되지 않아 기본 장비안과 확장 장비안을 모두 받아두기로 했습니다.'
	},
	{
		id: 'activity-delta-impact-lost',
		taskID: 'crm-task-delta-impact-lost',
		organizationID: 'organization-delta-impact',
		kind: 'stage_change',
		title: '이번 라운드 참여 보류',
		occurredAt: '2026-06-18T17:10:00+09:00',
		summary: '현재 투자 기준에는 맞지 않지만 분기 업데이트는 계속 받겠다고 답변했습니다.'
	},
	{
		id: 'activity-design-lab-reconnect',
		taskID: 'crm-task-design-lab-reconnect',
		organizationID: 'organization-design-lab-one',
		kind: 'note',
		title: '재접촉 후보로 표시',
		occurredAt: '2026-05-28T09:20:00+09:00',
		summary: '하반기 브랜딩 예산이 확정되면 다시 견적을 요청하기로 했습니다.'
	}
];

export const crmActivities: CRMActivity[] = crmActivityFixtures.map((activity) => {
	const opportunity = crmOpportunities.find((candidate) => candidate.organizationID === activity.organizationID);
	return {
		...activity,
		opportunityID: opportunity?.id,
		business: opportunity?.business ?? defaultCRMBusiness
	};
});

export const crmIntakeDrafts: CRMIntakeDraft[] = [
	{
		id: 'draft-mail-seoul-impact',
		source: 'mail',
		title: '메일에서 파일럿 미팅 요청 감지',
		organizationName: '샘플 임팩트 랩',
		summary: '김테스트25 리드가 다음 주 수요일 데모 가능 여부와 예상 비용 범위를 요청했습니다.',
		confidence: 91,
		suggestedAction: '다음 액션으로 데모 일정 확정 추가'
	},
	{
		id: 'draft-file-blue-campus',
		source: 'file',
		title: '후원 제안서 PDF에서 금액 후보 추출',
		organizationName: '예시캠퍼스 재단',
		summary: '하반기 후원 패키지 1,500만 원과 참여 학생 리포트 제공 조건이 발견되었습니다.',
		confidence: 84,
		suggestedAction: '진행 건 금액과 세부 메모 업데이트'
	},
	{
		id: 'draft-calendar-hanyang',
		source: 'calendar',
		title: '캘린더 회의록 초안 생성',
		organizationName: '예시 스타트업 지원단',
		summary: '멘토 12명 기준 운영 범위와 계약서 수정 요청이 회의 제목과 참석자에서 확인되었습니다.',
		confidence: 76,
		suggestedAction: '활동 기록에 회의 메모 추가'
	},
	{
		id: 'draft-file-ai-learning',
		source: 'file',
		title: '샘플 CSV에서 신규 관계처 후보 감지',
		organizationName: '샘플러닝랩',
		summary: '교육 운영팀, 담당자 3명, 예상 파일럿 금액 2,800만 원이 같은 파일에서 추출되었습니다.',
		confidence: 88,
		suggestedAction: '진행 건과 외부 담당자 후보로 저장'
	},
	{
		id: 'draft-mail-root-finance',
		source: 'mail',
		title: '투자심사 자료 요청 분류',
		organizationName: '샘플파이낸스',
		summary: '월간 매출, 리텐션, 운영 비용 자료 요청이 투자 검토 진행 건과 연결되었습니다.',
		confidence: 86,
		suggestedAction: '다음 액션에 데이터룸 업로드 추가'
	},
	{
		id: 'draft-calendar-k-edu',
		source: 'calendar',
		title: '구매 검토 회의 참석자 매칭',
		organizationName: '예시에듀 네트워크',
		summary: 'DX 추진팀과 보안 담당자가 참석한 회의가 운영 대시보드 도입 건으로 연결되었습니다.',
		confidence: 79,
		suggestedAction: '활동 기록에 구매 검토 미팅 추가'
	},
	{
		id: 'draft-mail-green-catering',
		source: 'mail',
		title: '케이터링 단가표 첨부 감지',
		organizationName: '예시케이터링',
		summary: '도시락, 핑거푸드, 음료 패키지 금액과 최소 주문 수량이 첨부파일에서 확인되었습니다.',
		confidence: 82,
		suggestedAction: '진행 건 금액과 파일 기록 업데이트'
	},
	{
		id: 'draft-file-city-office',
		source: 'file',
		title: '공공기관 제안서 양식 확인',
		organizationName: '예시구청 청년정책과',
		summary: '개인정보 처리 항목과 내부 심의 일정이 제안서 양식에서 추출되었습니다.',
		confidence: 74,
		suggestedAction: '관계처 주의사항과 다음 액션에 반영'
	}
];

const openStages: CRMOpportunityStage[] = ['waiting', 'in_progress', 'review', 'on_hold'];

function buildStageCounts(opportunities: CRMOpportunity[]): Record<string, number> {
	const stageCounts: Record<string, number> = {
		waiting: 0,
		in_progress: 0,
		review: 0,
		done: 0,
		lost: 0,
		on_hold: 0
	};

	for (const opportunity of opportunities) {
		stageCounts[opportunity.stage] += 1;
	}

	return stageCounts;
}

function buildOwnerSummaries(organizations: CRMOrganization[], opportunities: CRMOpportunity[]): CRMReportSummary['ownerSummaries'] {
	const ownerNames = [...new Set(organizations.map((organization) => organization.ownerName))];

	return ownerNames.map((ownerName) => {
		const ownerOrganizations = organizations.filter((organization) => organization.ownerName === ownerName);
		const ownerOpportunities = opportunities.filter((opportunity) => opportunity.ownerName === ownerName && openStages.includes(opportunity.stage));
		const missingActionCount = ownerOpportunities.filter((opportunity) => !opportunity.nextActionID).length;

		return {
			ownerName,
			organizationCount: ownerOrganizations.length,
			openOpportunityCount: ownerOpportunities.length,
			missingActionCount
		};
	});
}

export const crmReportSummary: CRMReportSummary = {
	stageCounts: buildStageCounts(crmOpportunities),
	ownerSummaries: buildOwnerSummaries(crmOrganizations, crmOpportunities)
};
