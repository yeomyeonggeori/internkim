export type LocalLLMBackendStatus = {
	name: string;
	model?: string;
	available: boolean;
	lastError?: string;
	lastCheckedAt?: string;
};

export type LocalLLMStatus = {
	enabled: boolean;
	backends?: LocalLLMBackendStatus[];
};

export type RuntimeStatus = {
	isRunning: boolean;
	processID?: number;
	lastError?: string;
	lastHeartbeatAt?: string;
	restartAttempts?: number;
	localLLM?: LocalLLMStatus;
};

export const runtimeState = $state<RuntimeStatus>({ isRunning: false });

export function setRuntimeState(nextStatus: RuntimeStatus) {
	runtimeState.isRunning = nextStatus.isRunning;
	runtimeState.processID = nextStatus.processID;
	runtimeState.lastError = nextStatus.lastError;
	runtimeState.lastHeartbeatAt = nextStatus.lastHeartbeatAt;
	runtimeState.restartAttempts = nextStatus.restartAttempts;
	runtimeState.localLLM = nextStatus.localLLM;
}
