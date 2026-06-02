// Flow 보고 탭 테스트용 summary fixture를 제공합니다.
export type FlowReportFixtureMetrics = {
	totalTasks: number;
	completedTasks: number;
	requestedTasks: number;
	pausedTasks: number;
	stoppedTasks: number;
	totalScore: number;
	statusCounts: Record<string, number>;
	businessCounts: Record<string, number>;
	typeCounts: Record<string, number>;
	memberScores: Record<string, number>;
};

export const flowReportFixtureMetrics: FlowReportFixtureMetrics = {
	totalTasks: 14,
	completedTasks: 5,
	requestedTasks: 2,
	pausedTasks: 1,
	stoppedTasks: 1,
	totalScore: 27,
	statusCounts: {
		완료: 5,
		진행: 4,
		요청: 2,
		예정: 2,
		일시정지: 1,
		중단: 0
	},
	businessCounts: {
		개발: 7,
		운영: 4,
		기획: 3
	},
	typeCounts: {
		구현: 6,
		회의: 3,
		문서: 2,
		검증: 3
	},
	memberScores: {
		김여명: 11,
		박예시: 7,
		최견본: 5,
		정의: 4,
		장석민: 0
	}
};
