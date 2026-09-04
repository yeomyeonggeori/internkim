import type { TaskReportSnapshot } from './report/task-report-data';

export type TaskWeek = {
	code: string;
	startISO: string;
	endISO: string;
	previous: string;
	next: string;
	isCurrent: boolean;
};

export type TaskMember = {
	id: string;
	name: string;
	email: string;
	image?: string;
	hireDate?: string;
	role: string;
	distance?: number;
	score?: number;
	activeTaskCount: number;
	completeTaskCount: number;
};

export type Task = {
	id: string;
	parentTaskID?: string;
	ownerID: string;
	ownerName: string;
	participantIDs: string[];
	participantNames: string[];
	requesterID?: string;
	requesterName?: string;
	business: string | null;
	type: string | null;
	content: string;
	size: string;
	status: string;
	startDate?: string;
	endDate?: string;
	createdAt?: string;
	weekCode: string;
};

export type TaskQuickTaskCreateResult = 'created' | 'duplicate' | 'failed' | 'ignored';

export type TaskMetrics = {
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
	memberScoreDetails?: Record<string, TaskMemberScoreDetail>;
};

export type TaskMemberScoreDetail = {
	weeklyScore: number;
	monthlyScore: number;
	currentScore: number;
};

export type TaskSizeDefinition = {
	name: string;
	distanceKm: number;
	maxHours: number;
	developmentExample: string;
	otherExample: string;
	note: string;
	score: number;
	label: string;
};

export type TaskDefinitions = {
	categories: string[];
	categoryColors?: Record<string, string>;
	types: string[];
	typeColors?: Record<string, string>;
	etcBusinessColor?: string;
	etcTypeColor?: string;
	sizes: TaskSizeDefinition[];
};

export type TaskSummary = {
	week: TaskWeek;
	currentWeek?: TaskWeek;
	members: TaskMember[];
	tasks: Task[];
	weeklyTasks?: Task[];
	metrics: TaskMetrics;
	definitions: TaskDefinitions;
	report?: TaskReportSnapshot;
	statusOptions: string[];
	currentUserEmail: string;
	currentUserName: string;
	isAdmin: boolean;
};

export type TaskWeeklySummary = {
	week: TaskWeek;
	currentWeek?: TaskWeek;
	weeklyTasks: Task[];
	metrics: TaskMetrics;
	report?: TaskReportSnapshot;
};

export type TaskState = {
	currentWeek?: TaskWeek;
	members: TaskMember[];
	tasks: Task[];
	metrics: TaskMetrics;
	definitions: TaskDefinitions;
	statusOptions: string[];
	currentUserEmail: string;
	currentUserName: string;
	isAdmin: boolean;
};
