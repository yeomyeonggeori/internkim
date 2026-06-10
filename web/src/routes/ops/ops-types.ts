export type OpsTarget = {
	id: string;
	name: string;
	adminURL: string;
	profile?: string;
	nodeArgument?: string;
	nodeID?: string;
	statePath?: string;
	secretSource?: string;
};

export type EndpointStatus = {
	state: string;
	code?: number;
	message?: string;
	startedAt?: string;
	release?: string;
	currentRelease?: string;
	latestRelease?: string;
	updateAllowed?: boolean;
};

export type VersionStatus = {
	admind?: string;
	runtime?: string;
	web?: string;
	current?: ReleaseVersion;
	latest?: ReleaseVersion;
};

export type ReleaseVersion = {
	releaseID?: string;
	admind?: string;
	capabilityd?: string;
	blueclawPayload?: string;
	skills?: string;
	runtime?: string;
	web?: string;
	components?: Record<string, ComponentBrief>;
};

export type ComponentBrief = {
	revision?: string;
	sha256?: string;
};

export type RecoveryStatus = {
	state: string;
	message?: string;
	services?: Record<string, string>;
};

export type LLMModelStatus = {
	state: string;
	model?: string;
	message?: string;
	runtimePath?: string;
	updatedAt?: string;
	restarted?: boolean;
};

export type TargetStatus = {
	targetID: string;
	checkedAt: string;
	admin: EndpointStatus;
	mattermost: EndpointStatus;
	release: EndpointStatus;
	recovery: RecoveryStatus;
	llm: LLMModelStatus;
	versions: VersionStatus;
};

export type Job = {
	id: string;
	targetID: string;
	action: string;
	state: string;
	startedAt: string;
	finishedAt?: string;
	error?: string;
	events?: JobEvent[];
};

export type JobEvent = {
	id: number;
	jobID: string;
	at: string;
	level: string;
	message: string;
};

export type LocalFleetStatus = {
	checkedAt: string;
	virtualMachine: EndpointStatus;
	ssh: EndpointStatus;
	admin: EndpointStatus;
	mattermost: EndpointStatus;
	adminURL?: string;
	mattermostURL?: string;
	lastResult?: string;
	cleanupNeeded: boolean;
	statePath?: string;
};

export type LocalFleetJobRequest = {
	action: string;
	recipe?: string;
	scenario?: string;
	base?: string;
};

export type NewTarget = {
	name: string;
	adminURL: string;
	profile?: string;
	nodeArgument?: string;
	secretSource?: string;
};
