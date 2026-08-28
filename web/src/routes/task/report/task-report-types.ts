export type TaskReportMetrics = {
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
	memberScoreDetails?: Record<string, TaskReportMemberScoreDetail>;
};

export type TaskReportMemberScoreDetail = {
	weeklyScore: number;
	monthlyScore: number;
	currentScore: number;
};

export type TaskReportSectionID = 'weeklyStatus' | 'memberDistance' | 'weeklyDistanceTrend' | 'monthlyDistanceTrend' | 'businessDistance';
export type TaskReportChartKind = 'lineComparison' | 'donut' | 'memberScoreList' | 'dailyTypeStacked';

export type TaskReportSectionLabel = {
	title: string;
	description: string;
};

export type TaskReportSectionLabels = Record<TaskReportSectionID, TaskReportSectionLabel>;

export type TaskReportCopy = {
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

export type TaskReportOptions = {
	emptyLabel: string;
	sectionLabels: TaskReportSectionLabels;
	copy: TaskReportCopy;
	report?: TaskReportSnapshot;
	tasks?: TaskReportTask[];
	members?: TaskReportMember[];
	definitions?: TaskReportDefinitions;
	weekStartISO?: string;
};

export type TaskReportMember = {
	id: string;
	name: string;
};

export type TaskReportItem = {
	label: string;
	description: string;
	value: number;
	percent: number;
	tone: TaskReportTone;
	colorIndex?: number;
};

export type TaskReportTone = 'business' | 'type';

export type TaskReportSegment = {
	label: string;
	value: number;
	percent: number;
	tone: TaskReportTone;
	colorIndex: number;
};

export type TaskReportRow = {
	label: string;
	total: number;
	percent: number;
	summary?: string;
	segments: TaskReportSegment[];
};

export type TaskReportTrend = {
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

export type TaskReportSnapshot = {
	weeklyDistanceTrend: TaskReportTrend;
	monthlyDistanceTrend: TaskReportTrend;
};

type TaskReportSectionData = {
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
	items: TaskReportItem[];
	rows: TaskReportRow[];
	trend: TaskReportTrend;
};

type TaskReportSectionWith<SectionID extends TaskReportSectionID, ChartKind extends TaskReportChartKind> = TaskReportSectionData & {
	id: SectionID;
	chartKind: ChartKind;
};

export type TaskDailyTypeDistanceSection = TaskReportSectionWith<'weeklyStatus', 'dailyTypeStacked'>;
export type TaskBusinessDistanceSection = TaskReportSectionWith<'businessDistance', 'donut'>;
export type TaskMemberScoreSection = TaskReportSectionWith<'memberDistance', 'memberScoreList'>;
export type TaskTrendSection = TaskReportSectionWith<'weeklyDistanceTrend' | 'monthlyDistanceTrend', 'lineComparison'>;
export type TaskChartSection = TaskDailyTypeDistanceSection | TaskBusinessDistanceSection | TaskTrendSection;
export type TaskReportSection = TaskChartSection | TaskMemberScoreSection;

export type TaskReportSections = {
	weeklyStatus: TaskDailyTypeDistanceSection;
	memberDistance: TaskMemberScoreSection;
	weeklyDistanceTrend: TaskTrendSection;
	monthlyDistanceTrend: TaskTrendSection;
	businessDistance: TaskBusinessDistanceSection;
};

export type TaskReportTask = {
	participantNames: string[];
	business: string;
	type: string;
	size: string;
	status: string;
	startDate?: string;
	endDate?: string;
};

export type TaskReportDefinitions = {
	categories: string[];
	types: string[];
	sizes: Array<{
		name: string;
		distanceKm: number;
	}>;
};
