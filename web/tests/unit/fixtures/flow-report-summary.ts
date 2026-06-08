export type FlowReportFixtureMetrics = {
	totalTasks: number;
	completedTasks: number;
	requestedTasks: number;
	pausedTasks: number;
	stoppedTasks: number;
	totalDistance: number;
	totalScore: number;
	statusCounts: Record<string, number>;
	businessCounts: Record<string, number>;
	typeCounts: Record<string, number>;
	memberDistances: Record<string, number>;
	memberScores: Record<string, number>;
	memberScoreDetails: Record<string, FlowReportFixtureMemberScoreDetail>;
};

export type FlowReportFixtureMemberScoreDetail = {
	weeklyScore: number;
	monthlyScore: number;
	currentScore: number;
};

export type FlowReportFixtureMember = {
	id: string;
	name: string;
};

export type FlowReportFixtureTask = {
	participantNames: string[];
	business: string;
	type: string;
	size: string;
	status: string;
	startDate?: string;
	endDate?: string;
};

export type FlowReportFixtureDefinitions = {
	categories: string[];
	types: string[];
	sizes: Array<{
		name: string;
		distanceKm: number;
	}>;
};

export type FlowReportFixtureSnapshot = {
	weeklyDistanceTrend: {
		labels: string[];
		currentLabel: string;
		previousLabel: string;
		currentValues: number[];
		previousValues: number[];
		currentTotal: number;
		previousTotal: number;
		unit: string;
	};
	monthlyDistanceTrend: {
		labels: string[];
		currentLabel: string;
		previousLabel: string;
		currentValues: number[];
		previousValues: number[];
		currentTotal: number;
		previousTotal: number;
		unit: string;
	};
};

export const flowReportFixtureMetrics: FlowReportFixtureMetrics = {
	totalTasks: 14,
	completedTasks: 5,
	requestedTasks: 2,
	pausedTasks: 1,
	stoppedTasks: 1,
	totalDistance: 7,
	totalScore: 279,
	statusCounts: {
		완료: 5,
		진행: 4,
		요청: 2,
		예정: 1,
		일시정지: 1,
		중단: 1
	},
	businessCounts: {
		여명거리: 7,
		김인턴: 7
	},
	typeCounts: {
		구현: 6,
		회의: 3,
		문서: 2,
		검증: 3
	},
	memberDistances: {
		김여명: 5,
		박예시: 2,
		최견본: 0,
		정의: 0,
		장석민: 0
	},
	memberScores: {
		'member-kim': 144,
		'member-park': 135,
		'member-lee': 0,
		'member-jeong': 0,
		'member-jang': 0
	},
	memberScoreDetails: {
		'member-kim': { weeklyScore: 115, monthlyScore: 173, currentScore: 144 },
		'member-park': { weeklyScore: 155, monthlyScore: 115, currentScore: 135 },
		'member-lee': { weeklyScore: 0, monthlyScore: 0, currentScore: 0 },
		'member-jeong': { weeklyScore: 0, monthlyScore: 0, currentScore: 0 },
		'member-jang': { weeklyScore: 0, monthlyScore: 0, currentScore: 0 }
	}
};

export const flowReportFixtureMembers: FlowReportFixtureMember[] = [
	{ id: 'member-kim', name: '김여명' },
	{ id: 'member-park', name: '박예시' },
	{ id: 'member-lee', name: '최견본' },
	{ id: 'member-jeong', name: '정의' },
	{ id: 'member-jang', name: '장석민' }
];

export const flowReportFixtureDefinitions: FlowReportFixtureDefinitions = {
	categories: ['여명거리', '김인턴'],
	types: ['구현', '회의', '검증', '문서'],
	sizes: [
		{ name: 'S', distanceKm: 2 },
		{ name: 'M', distanceKm: 3 },
		{ name: 'L', distanceKm: 5 },
		{ name: 'XL', distanceKm: 8 }
	]
};

export const flowReportFixtureTasks: FlowReportFixtureTask[] = [
	{ participantNames: ['김여명'], business: '여명거리', type: '구현', size: 'L', status: '완료', startDate: '2026-06-01', endDate: '2026-06-01' },
	{ participantNames: ['김여명', '박예시'], business: '여명거리', type: '검증', size: 'M', status: '진행', startDate: '2026-06-02' },
	{ participantNames: ['박예시'], business: '김인턴', type: '회의', size: 'S', status: '완료', startDate: '2026-06-03', endDate: '2026-06-03' },
	{ participantNames: ['최견본'], business: '김인턴', type: '문서', size: 'XL', status: '일시정지', startDate: '2026-06-04' }
];

export const flowReportFixtureSnapshot: FlowReportFixtureSnapshot = {
	weeklyDistanceTrend: {
		labels: ['1', '2', '3', '4', '5', '6', '7'],
		currentLabel: 'current',
		previousLabel: 'previous',
		currentValues: [0, 7, 10, 10, 15, 18, 18],
		previousValues: [0, 3, 6, 8, 8, 11, 11],
		currentTotal: 18,
		previousTotal: 11,
		unit: 'km'
	},
	monthlyDistanceTrend: {
		labels: ['1', '2', '3', '4', '5', '6', '7'],
		currentLabel: 'current',
		previousLabel: 'previous',
		currentValues: [0, 6, 9, 12, 15, 18, 18],
		previousValues: [0, 3, 6, 8, 10, 11, 11],
		currentTotal: 18,
		previousTotal: 11,
		unit: 'km'
	}
};
