import { invoke } from '@tauri-apps/api/core';
import type { Child } from '@tauri-apps/plugin-shell';
import { Command } from '@tauri-apps/plugin-shell';
import type { CompanionStatus, PairingPayload } from './pairing';
import { loadCompanionSettings, settingsToSidecarArguments } from './settings';
import type { CompanionSettings } from './settings';

export type RuntimeStatus = {
	isRunning: boolean;
	processID?: number;
	lastError?: string;
	lastHeartbeatAt?: string;
	restartAttempts?: number;
	localLLM?: LocalLLMStatus;
};

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

export type ResourceScope = {
	kind: string;
	value: string;
};

export type ActiveGrant = {
	grantID: string;
	displayName: string;
	capabilityScopes: string[];
	resourceScopes: ResourceScope[];
	usedJobs: number;
	maxJobs: number;
	expiresAt: string;
};

export type MountedFolder = {
	mountID: string;
	displayName: string;
	guestPath: string;
	mode: string;
	status: string;
	createdAt: string;
	lastSeenAt: string;
};

const controlURL = 'http://127.0.0.1:7983';

let runtimeChild: Child | undefined;
let runtimeStatus: RuntimeStatus = { isRunning: false };
let restartAttempts = 0;

type ShellBridgeInfo = {
	url: string;
	token: string;
};

export async function readCompanionStatus(): Promise<CompanionStatus> {
	const output = await Command.sidecar('binaries/internkim-companion', ['status', '--json']).execute();
	if (output.code !== 0) {
		return { paired: false };
	}
	return JSON.parse(output.stdout || '{"paired":false}') as CompanionStatus;
}

export async function pairCompanion(payload: PairingPayload): Promise<void> {
	const output = await Command.sidecar('binaries/internkim-companion', [
		'pair',
		'--device-url',
		payload.deviceURL,
		'--code',
		payload.code
	]).execute();
	if (output.code !== 0) {
		throw new Error(output.stderr || 'Pairing failed');
	}
}

export async function startCompanionRuntime(): Promise<void> {
	if (runtimeChild) return;
	const shellBridge = await invoke<ShellBridgeInfo>('start_shell_bridge');
	const settings = await loadCompanionSettings();
	const baseArguments = [
		'run',
		'--shell-bridge-url',
		shellBridge.url,
		'--shell-bridge-token',
		shellBridge.token,
		'--control-listen',
		'127.0.0.1:7983'
	];
	const command = Command.sidecar('binaries/internkim-companion', [
		...baseArguments,
		...settingsToSidecarArguments(settings)
	]);
	command.on('close', () => {
		runtimeChild = undefined;
		runtimeStatus = { isRunning: false, restartAttempts };
		void restartCompanionRuntimeOnce();
	});
	command.on('error', (errorValue) => {
		runtimeChild = undefined;
		runtimeStatus = { isRunning: false, lastError: errorValue, restartAttempts };
		void restartCompanionRuntimeOnce();
	});
	runtimeChild = await command.spawn();
	runtimeStatus = { isRunning: true, processID: runtimeChild.pid, restartAttempts };
}

export async function restartCompanionRuntime(): Promise<void> {
	if (!runtimeChild) {
		await startCompanionRuntime();
		return;
	}
	try {
		await runtimeChild.kill();
	} catch (error) {
		console.error('failed to stop companion runtime for restart', error);
	}
	runtimeChild = undefined;
	restartAttempts = 0;
	await startCompanionRuntime();
}

async function restartCompanionRuntimeOnce(): Promise<void> {
	if (restartAttempts >= 1) return;
	const status = await readCompanionStatus();
	if (!status.paired) return;
	restartAttempts += 1;
	window.setTimeout(() => {
		void startCompanionRuntime();
	}, 1200);
}

export function readRuntimeStatus(): RuntimeStatus {
	return runtimeStatus;
}

export async function refreshRuntimeStatus(): Promise<RuntimeStatus> {
	if (!runtimeChild) return runtimeStatus;
	try {
		const response = await fetch(`${controlURL}/v1/runtime/status`);
		if (!response.ok) return runtimeStatus;
		const document = (await response.json()) as {
			lastHeartbeatAt?: string;
			lastError?: string;
			localLLM?: LocalLLMStatus;
		};
		runtimeStatus = {
			...runtimeStatus,
			lastHeartbeatAt: document.lastHeartbeatAt,
			lastError: document.lastError || runtimeStatus.lastError,
			localLLM: document.localLLM
		};
		return runtimeStatus;
	} catch {
		return runtimeStatus;
	}
}

export async function updateRuntimeLocalLLM(settings: CompanionSettings): Promise<LocalLLMStatus | undefined> {
	if (!runtimeChild) return undefined;
	const response = await fetch(`${controlURL}/v1/runtime/local-llm`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(settings)
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	const localLLM = (await response.json()) as LocalLLMStatus;
	runtimeStatus = {
		...runtimeStatus,
		localLLM
	};
	return localLLM;
}

export async function readRemoteModel(): Promise<string> {
	const output = await Command.sidecar('binaries/internkim-companion', ['remote-model', 'get']).execute();
	if (output.code !== 0) {
		throw new Error(output.stderr || 'Remote model read failed');
	}
	return output.stdout.trim();
}

export async function updateRemoteModel(modelName: string): Promise<string> {
	const output = await Command.sidecar('binaries/internkim-companion', [
		'remote-model',
		'set',
		'--model',
		modelName
	]).execute();
	if (output.code !== 0) {
		throw new Error(output.stderr || 'Remote model update failed');
	}
	return output.stdout.trim();
}

export async function readActiveGrants(): Promise<ActiveGrant[]> {
	if (!runtimeChild) return [];
	const response = await fetch(`${controlURL}/v1/security/grants`);
	if (!response.ok) return [];
	const document = (await response.json()) as { grants?: ActiveGrant[] };
	return document.grants ?? [];
}

export async function revokeGrant(grantID: string): Promise<void> {
	const response = await fetch(`${controlURL}/v1/security/grants/${encodeURIComponent(grantID)}/revoke`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw new Error('Grant revoke failed');
	}
}

export async function readMountedFolders(): Promise<MountedFolder[]> {
	if (!runtimeChild) return [];
	const response = await fetch(`${controlURL}/v1/filesystem/mounts`);
	if (!response.ok) return [];
	const document = (await response.json()) as { mounts?: MountedFolder[] };
	return document.mounts ?? [];
}

export async function addMountedFolder(path: string): Promise<MountedFolder> {
	const response = await fetch(`${controlURL}/v1/filesystem/mounts`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ path })
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return (await response.json()) as MountedFolder;
}

export async function revokeMountedFolder(mountID: string): Promise<void> {
	const response = await fetch(`${controlURL}/v1/filesystem/mounts/${encodeURIComponent(mountID)}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}

export async function pauseMountedFolder(mountID: string): Promise<void> {
	const response = await fetch(`${controlURL}/v1/filesystem/mounts/${encodeURIComponent(mountID)}/pause`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}

export async function resumeMountedFolder(mountID: string): Promise<void> {
	const response = await fetch(`${controlURL}/v1/filesystem/mounts/${encodeURIComponent(mountID)}/resume`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}
