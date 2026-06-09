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
};

export type RecoveryStatus = {
	state: string;
	message?: string;
	services?: Record<string, string>;
};

export type TargetStatus = {
	targetID: string;
	checkedAt: string;
	admin: EndpointStatus;
	mattermost: EndpointStatus;
	release: EndpointStatus;
	recovery: RecoveryStatus;
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

export type NewTarget = {
	name: string;
	adminURL: string;
	profile?: string;
	nodeArgument?: string;
	secretSource?: string;
};
