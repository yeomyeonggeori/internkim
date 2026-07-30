import type { FlowReportSnapshot } from './report/flow-report-data';

export type FlowWeek = {
	code: string;
	startISO: string;
	endISO: string;
	previous: string;
	next: string;
	isCurrent: boolean;
};

export type FlowMember = {
	id: string;
	name: string;
	email: string;
	image?: string;
	hireDate?: string;
	role: string;
	mattermostStatus: string;
	distance?: number;
	score?: number;
	activeTaskCount: number;
	completeTaskCount: number;
};

export type FlowTask = {
	id: string;
	ownerID: string;
	ownerName: string;
	participantIDs: string[];
	participantNames: string[];
	business: string;
	type: string;
	content: string;
	goal: string;
	size: string;
	status: string;
	statusRank: number;
	startDate?: string;
	endDate?: string;
	createdAt?: string;
	weekCode: string;
	flag: number;
	requestReason?: string;
	decisionReason?: string;
};

export type FlowQuickTaskCreateResult = 'created' | 'duplicate' | 'failed' | 'ignored';

export type FlowMetrics = {
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
	memberScoreDetails?: Record<string, FlowMemberScoreDetail>;
};

export type FlowMemberScoreDetail = {
	weeklyScore: number;
	monthlyScore: number;
	currentScore: number;
};

export type FlowSizeDefinition = {
	name: string;
	distanceKm: number;
	maxHours: number;
	developmentExample: string;
	otherExample: string;
	note: string;
	color?: string;
	score: number;
	label: string;
};

export type FlowDefinitions = {
	categories: string[];
	categoryColors?: Record<string, string>;
	types: string[];
	typeColors?: Record<string, string>;
	sizes: FlowSizeDefinition[];
};

export type FlowSummary = {
	week: FlowWeek;
	currentWeek?: FlowWeek;
	members: FlowMember[];
	tasks: FlowTask[];
	weeklyTasks?: FlowTask[];
	metrics: FlowMetrics;
	definitions: FlowDefinitions;
	report?: FlowReportSnapshot;
	statusOptions: string[];
	currentUserEmail: string;
	currentUserName: string;
	isAdmin: boolean;
	source: string;
};

export type FlowWeeklySummary = {
	week: FlowWeek;
	currentWeek?: FlowWeek;
	weeklyTasks: FlowTask[];
	metrics: FlowMetrics;
	report?: FlowReportSnapshot;
	source: string;
};

export type FlowState = {
	currentWeek?: FlowWeek;
	members: FlowMember[];
	tasks: FlowTask[];
	metrics: FlowMetrics;
	definitions: FlowDefinitions;
	statusOptions: string[];
	currentUserEmail: string;
	currentUserName: string;
	isAdmin: boolean;
	source: string;
};
