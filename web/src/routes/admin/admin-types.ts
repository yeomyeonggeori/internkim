import type { adminText } from './text';

export type UserRole = 'admin' | 'member';
export type WorkspaceLanguage = 'ko' | 'en';
export type AdminSection = 'device' | 'bot' | 'credentials' | 'companion' | 'backup' | 'users' | 'settings';
export type AdminPageText = typeof adminText.ko;

export type UserRecord = {
	userID: string;
	handle: string;
	name?: string;
	email: string;
	hireDate?: string;
	role: UserRole;
	circles?: string[];
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

export type UsersResponse = {
	users?: string[];
	records?: UserRecord[];
	availableCircles?: CircleRecord[];
	temporaryPassword?: string;
	temporaryPasswordEmail?: string;
};

export type AdminSession = {
	email: string;
	claimedAdminEmail: string;
	isAdmin: boolean;
	isClaimed: boolean;
	bootstrapStatus: string;
	bootstrapError?: string;
	temporaryPassword?: string;
	temporaryPasswordEmail?: string;
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

export type RestoreUploadResponse = {
	uploadID: string;
	chunkSize: number;
};

export type CompanionCapability = {
	name: string;
	version?: string;
	privacyClass?: string;
	requiresUserPresence?: boolean;
	worksOffline?: boolean;
};

export type CompanionStatus = {
	companionID: string;
	displayName: string;
	capabilities?: CompanionCapability[];
	localOnly?: boolean;
	isOnline?: boolean;
	lastSeenAt?: string;
	disabled?: boolean;
};

export type CompanionStatusResponse = {
	companions?: CompanionStatus[];
};

export type CompanionPairingCodeResponse = {
	code: string;
	expiresAt: string;
	deepLink: string;
};

export type CompanionRelease = {
	platform: string;
	label: string;
	architecture: string;
	status?: string;
	url: string;
};

export type CompanionReleaseResponse = {
	platforms?: CompanionRelease[];
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
