// Flow 보고 섹션 타입을 정의합니다.
export type FlowReportMetrics = {
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

export type FlowReportSectionID = 'weeklyStatus' | 'memberDistance' | 'weeklyDistanceTrend' | 'monthlyDistanceTrend' | 'businessDistance';
export type FlowReportChartKind = 'lineComparison' | 'donut' | 'memberTypeStacked' | 'dailyTypeStacked';

export type FlowReportSectionLabel = {
	title: string;
	description: string;
};

export type FlowReportSectionLabels = Record<FlowReportSectionID, FlowReportSectionLabel>;

export type FlowReportOptions = {
	emptyLabel: string;
	sectionLabels: FlowReportSectionLabels;
	report?: FlowReportSnapshot;
	tasks?: FlowReportTask[];
	definitions?: FlowReportDefinitions;
	weekStartISO?: string;
};

export type FlowReportItem = {
	label: string;
	description: string;
	value: number;
	percent: number;
	tone: FlowReportTone;
	colorIndex?: number;
};

export type FlowReportTone = 'business' | 'type';

export type FlowReportSegment = {
	label: string;
	value: number;
	percent: number;
	tone: FlowReportTone;
	colorIndex: number;
};

export type FlowReportRow = {
	label: string;
	total: number;
	percent: number;
	segments: FlowReportSegment[];
};

export type FlowReportTrend = {
	labels: string[];
	currentLabel: string;
	previousLabel: string;
	currentValues: number[];
	previousValues: number[];
	currentTotal: number;
	previousTotal: number;
	unit: string;
};

export type FlowReportSnapshot = {
	weeklyDistanceTrend: FlowReportTrend;
	monthlyDistanceTrend: FlowReportTrend;
};

export type FlowReportSection = {
	id: FlowReportSectionID;
	chartKind: FlowReportChartKind;
	title: string;
	description: string;
	unit: string;
	total: number;
	maxValue: number;
	averageValue: number;
	alertValue: number;
	emptyLabel: string;
	items: FlowReportItem[];
	rows: FlowReportRow[];
	trend: FlowReportTrend;
};

export type FlowReportSections = Record<FlowReportSectionID, FlowReportSection>;

export type FlowReportTask = {
	participantNames: string[];
	business: string;
	type: string;
	size: string;
	status: string;
	startDate?: string;
	endDate?: string;
};

export type FlowReportDefinitions = {
	categories: string[];
	types: string[];
	sizes: Array<{
		name: string;
		distanceKm: number;
	}>;
};
