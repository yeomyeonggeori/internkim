export type FlowReportMetrics = {
	totalTasks: number;
	completedTasks: number;
	requestedTasks: number;
	pausedTasks: number;
	stoppedTasks: number;
	totalDistance?: number;
	totalScore?: number;
	statusCounts: Record<string, number>;
	businessCounts: Record<string, number>;
	typeCounts: Record<string, number>;
	memberDistances?: Record<string, number>;
	memberScores?: Record<string, number>;
	memberScoreDetails?: Record<string, FlowReportMemberScoreDetail>;
};

export type FlowReportMemberScoreDetail = {
	weeklyScore: number;
	monthlyScore: number;
	currentScore: number;
};

export type FlowReportSectionID = 'weeklyStatus' | 'memberDistance' | 'weeklyDistanceTrend' | 'monthlyDistanceTrend' | 'businessDistance';
export type FlowReportChartKind = 'lineComparison' | 'donut' | 'memberScoreList' | 'dailyTypeStacked';

export type FlowReportSectionLabel = {
	title: string;
	description: string;
};

export type FlowReportSectionLabels = Record<FlowReportSectionID, FlowReportSectionLabel>;

export type FlowReportCopy = {
	weekdays: string[];
	fallbackType: string;
	fallbackBusiness: string;
	memberScoreLabel: string;
	weeklyScoreLabel: string;
	monthlyScoreLabel: string;
	scoreUnit: string;
	teamAverageLabel: string;
	memberScrollHint: string;
	currentWeekTrend: string;
	previousWeekTrend: string;
	currentMonthTrend: string;
	previousMonthTrend: string;
	monthlyDayLabelTemplate: string;
};

export type FlowReportOptions = {
	emptyLabel: string;
	sectionLabels: FlowReportSectionLabels;
	copy: FlowReportCopy;
	report?: FlowReportSnapshot;
	tasks?: FlowReportTask[];
	members?: FlowReportMember[];
	definitions?: FlowReportDefinitions;
	weekStartISO?: string;
};

export type FlowReportMember = {
	id: string;
	name: string;
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
	summary?: string;
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
	labelTemplate?: string;
};

export type FlowReportSnapshot = {
	weeklyDistanceTrend: FlowReportTrend;
	monthlyDistanceTrend: FlowReportTrend;
};

type FlowReportSectionData = {
	title: string;
	description: string;
	unit: string;
	total: number;
	maxValue: number;
	averageValue: number;
	alertValue: number;
	emptyLabel: string;
	teamAverageLabel: string;
	memberScrollHint: string;
	items: FlowReportItem[];
	rows: FlowReportRow[];
	trend: FlowReportTrend;
};

type FlowReportSectionWith<SectionID extends FlowReportSectionID, ChartKind extends FlowReportChartKind> = FlowReportSectionData & {
	id: SectionID;
	chartKind: ChartKind;
};

export type FlowDailyTypeDistanceSection = FlowReportSectionWith<'weeklyStatus', 'dailyTypeStacked'>;
export type FlowBusinessDistanceSection = FlowReportSectionWith<'businessDistance', 'donut'>;
export type FlowMemberScoreSection = FlowReportSectionWith<'memberDistance', 'memberScoreList'>;
export type FlowTrendSection = FlowReportSectionWith<'weeklyDistanceTrend' | 'monthlyDistanceTrend', 'lineComparison'>;
export type FlowChartSection = FlowDailyTypeDistanceSection | FlowBusinessDistanceSection | FlowTrendSection;
export type FlowReportSection = FlowChartSection | FlowMemberScoreSection;

export type FlowReportSections = {
	weeklyStatus: FlowDailyTypeDistanceSection;
	memberDistance: FlowMemberScoreSection;
	weeklyDistanceTrend: FlowTrendSection;
	monthlyDistanceTrend: FlowTrendSection;
	businessDistance: FlowBusinessDistanceSection;
};

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
