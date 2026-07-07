import type { adminText } from './text';

export type UserRole = 'admin' | 'operationsAdmin' | 'member';
export type WorkspaceLanguage = 'ko' | 'en';
export type AdminSection = 'device' | 'bot' | 'credentials' | 'backup' | 'users' | 'orgchart' | 'settings' | 'network';
export type AdminPageText = typeof adminText.ko;

export type UserRecord = {
	userID: string;
	handle: string;
	name?: string;
	email: string;
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
