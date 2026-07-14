import type { adminText } from './text';

export type UserRole = 'admin' | 'operationsAdmin' | 'member';
export type WorkspaceLanguage = 'ko' | 'en';
export type AdminSection = 'device' | 'bot' | 'credentials' | 'backup' | 'users' | 'settings' | 'sharing' | 'network';
export type AdminPageText = typeof adminText.ko;

export type UserRecord = {
	userID: string;
	handle: string;
	name?: string;
	email: string;
	image?: string;
	hireDate?: string;
	note?: string;
	role: UserRole;
	circles?: string[];
	jobTitle?: string;
	group?: string;
	primaryGroupID?: string;
	groupIDs?: string[];
	supervisorID?: string;
	mattermostUserID?: string;
	mattermostUsername?: string;
	status?: string;
	isIncomplete?: boolean;
};

export type CircleRecord = {
	circleID: string;
	displayName: string;
	isMattermostManaged?: boolean;
};

export type OrgGroup = {
	id: string;
	name: string;
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
	isPocSuperAdmin?: boolean;
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
	mattermostDump?: boolean;
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

export type BotProfile = {
	username: string;
	displayName: string;
	englishDisplayName?: string;
	aliases?: string[];
	publicDescription: string;
	identityExtension?: string;
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

export type AttendanceLocation = {
	id: string;
	name: string;
	color: string;
	isDefault: boolean;
	updatedAt?: string;
};

export type AttendanceLocationsResponse = {
	locations?: AttendanceLocation[];
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
	metric: string;
	year: number;
	quarter?: number;
	month?: number;
	value: number;
	currency?: CompanyMetricCurrency;
	valueUSD?: number;
	unit?: string;
	note?: string;
};

export type CompanyMetricCurrency = 'USD' | 'KRW' | 'EUR' | 'JPY' | 'GBP' | 'CNY' | 'HKD' | 'SGD' | 'AUD' | 'CAD' | 'CHF' | 'INR';

export type CompanyRecord = {
	id: string;
	category: string;
	date?: string;
	title: string;
	detail?: string;
	attributes?: Record<string, unknown>;
};

export type CompanyDocument = {
	id: string;
	documentType: string;
	title: string;
	language?: string;
	summary?: string;
	issuedAt?: string;
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

export type CompanyMetricsResponse = {
	metrics?: CompanyMetric[];
};

export type CompanyRecordsResponse = {
	records?: CompanyRecord[];
};

export type CompanyDocumentsResponse = {
	documents?: CompanyDocument[];
};
