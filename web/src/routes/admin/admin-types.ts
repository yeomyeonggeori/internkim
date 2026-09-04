import type { adminText } from './text';

import type { OrgGroup, UserRecord } from '$lib/organization/types';
import type { UserRole } from '$lib/types';
import type { PageText } from '$lib/i18n/page-text.svelte';

export type { OrgGroup, UserRecord } from '$lib/organization/types';
export type { UserRole } from '$lib/types';
export type WorkspaceLanguage = 'ko' | 'en';
export type AdminSection =
	| 'device'
	| 'bot'
	| 'credentials'
	| 'backup'
	| 'users'
	| 'settings'
	| 'workSettings'
	| 'leaveSettings'
	| 'sharing'
	| 'network'
	| 'buzz'
	| 'apiTokens';

export type BuzzInviteRecord = {
	code: string;
	name: string;
	email: string;
	createdAt: string;
	expiresAt: string;
	claimedPubkey?: string;
};
export type AdminPageText = PageText<typeof adminText>;

export type CircleRecord = {
	circleID: string;
	displayName: string;
	isMattermostManaged?: boolean;
};

export type UsersResponse = {
	users?: string[];
	records?: UserRecord[];
	availableCircles?: CircleRecord[];
	availableGroups?: OrgGroup[];
	temporaryPassword?: string;
	temporaryPasswordEmail?: string;
};

export type AdminSession = {
	email: string;
	image?: string;
	claimedAdminEmail: string;
	isAdmin: boolean;
	role?: UserRole;
	canViewTasks?: boolean;
	isClaimed: boolean;
	bootstrapStatus: string;
	bootstrapError?: string;
	temporaryPassword?: string;
	temporaryPasswordEmail?: string;
	mattermostURL?: string;
	deviceManaged?: boolean;
};

export type BackupManifest = {
	fleetID?: string;
	createdAt?: string;
	components?: string[];
};

export type AdminJob = {
	jobID: string;
	type: string;
	status: string;
	phase: string;
	error?: string;
	downloadURL?: string;
	manifest?: BackupManifest;
	logs?: string[];
};

export type ReleaseUpdateSummary = {
	releaseID: string;
	channel?: string;
	createdAt?: string;
	components?: Record<string, { revision: string; sha256?: string }>;
};

export type BlueclawUpdateStatus = {
	current?: ReleaseUpdateSummary;
	latest?: ReleaseUpdateSummary;
	state: string;
	updateAllowed: boolean;
	activeJob?: AdminJob;
};

export type ReleaseHistoryEntry = {
	releaseID: string;
	manifestURL: string;
	createdAt: string;
	isCurrent: boolean;
};

export type ReleaseHistoryResponse = {
	entries?: ReleaseHistoryEntry[];
};

export type RestoreUploadResponse = {
	uploadID: string;
	chunkSize: number;
};

export type AgentToneRegister = 'formal' | 'polite' | 'casual';

export type AgentSoul = {
	schemaVersion: 1;
	values?: string[];
	boundaries?: string[];
	workingStyle?: string[];
	tone?: { register?: AgentToneRegister; traits?: string[] };
	language?: { default?: string; matchRequester?: boolean };
};

export type AgentUser = {
	schemaVersion: 1;
	callMe?: string;
	about?: string;
	preferences?: string[];
	tone?: { register?: AgentToneRegister; traits?: string[] };
	language?: { default?: string };
};

export type CredentialProviderStatus = {
	provider: string;
	configured: boolean;
	fingerprint?: string;
};

export type CredentialProvidersResponse = {
	providers?: CredentialProviderStatus[];
};

export type WorkspaceSettings = {
	timeZone: string;
	language: WorkspaceLanguage;
	updatedAt?: string;
};

export type CalendarHolidaySyncState = 'healthy' | 'degraded' | 'neverSynced';

export type LeaveBalanceMode = 'annual' | 'separate' | 'none';
export type LeaveBalanceTrackingMode = 'managed' | 'unlimited';
export type LeaveGrantCadence = 'annual' | 'monthly' | 'none';
export type LeaveExpiryMode = 'fiscalYearEnd' | 'monthsAfterGrant' | 'none';
export type LeaveAllowedUnit = 'fullDay' | 'halfDay' | 'quarterDay';

export type LeaveType = {
	id: string;
	systemKind: string;
	name: string;
	paid: boolean;
	balanceMode: LeaveBalanceMode;
	grantCadence: LeaveGrantCadence;
	grantAmountMilliDays: number;
	expiryMode: LeaveExpiryMode;
	expiryMonths?: number;
	carryoverEnabled: boolean;
	carryoverLimitMilliDays?: number;
	allowedUnits: LeaveAllowedUnit[];
	includeInSummary: boolean;
	isActive: boolean;
	isSystem: boolean;
	sortOrder: number;
};

export type AttendanceLeavePolicy = {
	version: 2;
	balanceTrackingMode: LeaveBalanceTrackingMode;
	fiscalYearStartMonth: number;
	fiscalYearStartDay: number;
	leaveTypes: LeaveType[];
	updatedAt: string;
};

export type CompanyHoliday = {
	id: string;
	title: string;
	date: string;
	recursAnnually: boolean;
	createdAt: string;
	updatedAt: string;
};

export type CompanyHolidayInput = Pick<CompanyHoliday, 'title' | 'date' | 'recursAnnually'>;

export type CompanyHolidaysResponse = {
	holidays: CompanyHoliday[];
};

export type AttendanceWorkMode = 'autonomous' | 'flexible' | 'fixed';

export type AttendanceWorkBreakPeriod = {
	startTime: string;
	endTime: string;
};

export type AttendanceWorkPolicyRevision = {
	effectiveDate: string;
	workMode: AttendanceWorkMode;
	workingWeekdays: number[];
	dailyTargetMinutes: number;
	weeklyTargetMinutes: number;
	referenceStartTime: string;
	fixedStartTime: string;
	fixedEndTime: string;
	coreTimeEnabled: boolean;
	coreStartTime: string;
	coreEndTime: string;
	breakPeriods: AttendanceWorkBreakPeriod[];
	nightStartTime: string;
	nightEndTime: string;
};

export type AttendanceWorkPolicy = {
	version: 1;
	updatedAt: string;
	revisions: AttendanceWorkPolicyRevision[];
};

export type AttendanceWorkPolicyResponse = {
	policy: AttendanceWorkPolicy;
	currentMonth: string;
	holidayDates: string[];
	timeZone: string;
};

export type WifiProfile = {
	name: string;
	ssid: string;
	isActive: boolean;
	isManagedByInternkim: boolean;
};

export type WifiProfilesResponse = {
	profiles?: WifiProfile[];
};

export type CompanyMetric = {
	metricID: string;
	metric: string;
	year: number;
	quarter: number;
	month: number;
	value: number;
	currency: CompanyMetricCurrency | null;
	valueUSD: number | null;
	unit: string | null;
	note: string | null;
	updatedAt: string;
};

export type CompanyMetricCurrency = 'USD' | 'KRW' | 'EUR' | 'JPY' | 'GBP' | 'CNY' | 'HKD' | 'SGD' | 'AUD' | 'CAD' | 'CHF' | 'INR';

export type CompanyRecordAttribute = {
	label: string;
	value: string;
};

export type CompanyRecord = {
	recordID: string;
	category: string;
	date: string | null;
	title: string;
	detail: string | null;
	attributes: CompanyRecordAttribute[];
	updatedAt: string;
};

export type CompanyDocument = {
	documentID: string;
	documentNumber: string | null;
	kind: string;
	documentType: string;
	title: string;
	counterpart: string | null;
	language: string | null;
	filePath: string | null;
	summary: string | null;
	requesterID: string | null;
	issuedAt: string;
};

export type CompanyShareNarrative = {
	highlights: string[];
	businessModel: string;
	customerEvidence: string;
	marketOpportunity: string;
	competitiveAdvantage: string;
	roadmap: string;
	fundingStage: string;
	fundingTarget: string;
	useOfFunds: string;
};

export type CompanyShareMetricContext = {
	labels: Record<string, string>;
	descriptions: Record<string, string>;
	favorableDirection: 'increase' | 'decrease' | 'neutral';
	evidenceRole: '' | 'growth' | 'efficiency' | 'scale' | 'quality' | 'reach' | 'capital';
	showSource: boolean;
};

export type CompanyShareRecordContext = {
	titles: Record<string, string>;
	descriptions: Record<string, string>;
	attributeKeys: string[];
};

export type CompanyShareSettings = {
	enabled: boolean;
	hasPassword: boolean;
	sessionHours: number;
	languages: string[];
	profileFields: string[];
	metricNames: string[];
	primaryMetric?: string;
	metricContexts: Record<string, CompanyShareMetricContext>;
	recordIDs: string[];
	recordContexts: Record<string, CompanyShareRecordContext>;
	documentIDs: string[];
	contactEmail?: string;
	showTeamActivity: boolean;
	narratives: Record<string, CompanyShareNarrative>;
	publishedAt?: string;
	publicationRevision: number;
};

export type CompanyShareSettingsUpdate = Omit<CompanyShareSettings, 'hasPassword' | 'publishedAt' | 'publicationRevision'> & {
	password: string;
};

export type CompanyMetricListResult = {
	count: number;
	metrics: CompanyMetric[];
};

export type CompanyRecordListResult = {
	count: number;
	records: CompanyRecord[];
};

export type CompanyDocumentListResult = {
	count: number;
	documents: CompanyDocument[];
};
