import { invoke } from '@tauri-apps/api/core';
import { resolveResource } from '@tauri-apps/api/path';
import type { Child } from '@tauri-apps/plugin-shell';
import { Command } from '@tauri-apps/plugin-shell';
import { isCompanionVerified, stalePairingMessage, type CompanionStatus, type PairingPayload } from './pairing';
import { runtimeState, setRuntimeState, type RuntimeStatus, type LocalLLMStatus } from './runtimeState.svelte';
import { loadCompanionSettings } from './settings';
import type { CompanionSettings } from './settings';

export { runtimeState, setRuntimeState } from './runtimeState.svelte';
export type { LocalLLMBackendStatus, LocalLLMStatus, RuntimeStatus } from './runtimeState.svelte';

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

export const controlPlaneBaseURL = 'http://127.0.0.1:7983';
const controlPlaneListenAddress = '127.0.0.1:7983';

let runtimeChild: Child | undefined;
let restartAttempts = 0;
let shouldRestartRuntime = true;

type ShellBridgeInfo = {
	url: string;
	token: string;
};

export async function readCompanionStatus(): Promise<CompanionStatus> {
	const output = await Command.sidecar('binaries/internkim-companion', ['status', '--json', '--verify-auth']).execute();
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

export async function disconnectCompanion(): Promise<void> {
	await stopCompanionRuntime();
	const output = await Command.sidecar('binaries/internkim-companion', ['disconnect']).execute();
	shouldRestartRuntime = true;
	if (output.code !== 0) {
		throw new Error(output.stderr || 'Disconnect failed');
	}
	restartAttempts = 0;
	setRuntimeState({ isRunning: false });
}

export async function ensureLaunchAtLogin(): Promise<void> {
	await invoke('ensure_launch_at_login');
}

function localLLMSidecarArguments(settings: CompanionSettings): string[] {
	if (!settings.enableLocalLLM) return [];
	const flags = ['--enable-local-llm', '--local-backend-order', settings.localBackendOrder.join(',')];
	flags.push('--ollama-base-url', settings.ollama.baseURL);
	if (settings.ollama.model) {
		flags.push('--ollama-model', settings.ollama.model);
	}
	flags.push('--llamacpp-base-url', settings.llamacpp.baseURL);
	flags.push('--llamacpp-embedding-base-url', settings.llamacpp.baseURL);
	flags.push('--llamacpp-embedding-model', settings.llamacpp.embeddingModel || 'embeddinggemma');
	if (settings.llamacpp.model) {
		flags.push('--llamacpp-model', settings.llamacpp.model);
	}
	flags.push('--mlx-base-url', settings.mlx.baseURL);
	if (settings.mlx.model) {
		flags.push('--mlx-model', settings.mlx.model);
	}
	return flags;
}

async function buildSidecarArguments(): Promise<string[]> {
	const shellBridge = await invoke<ShellBridgeInfo>('start_shell_bridge');
	const settings = await loadCompanionSettings();
	const browserExtensionPath = await resolveResource('browser-extension');
	return [
		'run',
		'--shell-bridge-url',
		shellBridge.url,
		'--shell-bridge-token',
		shellBridge.token,
		'--control-listen',
		controlPlaneListenAddress,
		'--browser-extension-path',
		browserExtensionPath,
		...(settings.preferCompanionBrowser ? ['--prefer-companion-browser'] : []),
		...localLLMSidecarArguments(settings)
	];
}

export async function startCompanionRuntime(): Promise<void> {
	if (runtimeChild) return;
	shouldRestartRuntime = true;
	const sidecarArguments = await buildSidecarArguments();
	const command = Command.sidecar('binaries/internkim-companion', sidecarArguments);
	command.on('close', () => {
		runtimeChild = undefined;
		setRuntimeState({ isRunning: false, restartAttempts });
		if (!shouldRestartRuntime) return;
		void restartCompanionRuntimeOnce();
	});
	command.on('error', (errorValue) => {
		runtimeChild = undefined;
		setRuntimeState({ isRunning: false, lastError: errorValue, restartAttempts });
		if (!shouldRestartRuntime) return;
		void restartCompanionRuntimeOnce();
	});
	runtimeChild = await command.spawn();
	setRuntimeState({ isRunning: true, processID: runtimeChild.pid, restartAttempts });
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

export async function stopCompanionRuntime(): Promise<void> {
	shouldRestartRuntime = false;
	const child = runtimeChild;
	runtimeChild = undefined;
	if (!child) {
		setRuntimeState({ isRunning: false });
		return;
	}
	try {
		await child.kill();
	} catch (error) {
		console.error('failed to stop companion runtime', error);
	}
	setRuntimeState({ isRunning: false });
}

async function restartCompanionRuntimeOnce(): Promise<void> {
	if (!shouldRestartRuntime) return;
	if (restartAttempts >= 1) return;
	const status = await readCompanionStatus();
	if (!isCompanionVerified(status)) return;
	restartAttempts += 1;
	window.setTimeout(() => {
		void startCompanionRuntime();
	}, 1200);
}

export async function refreshRuntimeStatus(): Promise<RuntimeStatus> {
	if (!runtimeChild) return runtimeState;
	try {
		const response = await fetch(`${controlPlaneBaseURL}/v1/runtime/status`);
		if (!response.ok) return runtimeState;
		const document = (await response.json()) as {
			lastHeartbeatAt?: string;
			lastError?: string;
			localLLM?: LocalLLMStatus;
		};
		setRuntimeState({
			...runtimeState,
			lastHeartbeatAt: document.lastHeartbeatAt,
			lastError: document.lastError || runtimeState.lastError,
			localLLM: document.localLLM
		});
		return runtimeState;
	} catch {
		return runtimeState;
	}
}

export async function updateRuntimeLocalLLM(settings: CompanionSettings): Promise<LocalLLMStatus | undefined> {
	if (!runtimeChild) return undefined;
	const response = await fetch(`${controlPlaneBaseURL}/v1/runtime/local-llm`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(settings)
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	const localLLM = (await response.json()) as LocalLLMStatus;
	setRuntimeState({ ...runtimeState, localLLM });
	return localLLM;
}

export async function readRuntimeRemoteModel(): Promise<string> {
	const output = await Command.sidecar('binaries/internkim-companion', ['runtime-model', 'get']).execute();
	if (output.code !== 0) {
		throw new Error(normalizeCompanionSidecarError(output.stderr || 'Remote model read failed'));
	}
	return output.stdout.trim();
}

export async function updateRuntimeRemoteModel(modelName: string): Promise<string> {
	const output = await Command.sidecar('binaries/internkim-companion', [
		'runtime-model',
		'set',
		'--model',
		modelName
	]).execute();
	if (output.code !== 0) {
		throw new Error(normalizeCompanionSidecarError(output.stderr || 'Remote model update failed'));
	}
	return output.stdout.trim();
}

function normalizeCompanionSidecarError(message: string): string {
	if (message.includes('companion auth required')) return stalePairingMessage;
	if (message.includes('Pairing expired. Connect again from Admin.')) return stalePairingMessage;
	if (message.includes('Pairing expired. Run /connect to connect again.')) return message.trim();
	return message;
}

export async function readActiveGrants(): Promise<ActiveGrant[]> {
	if (!runtimeChild) return [];
	const response = await fetch(`${controlPlaneBaseURL}/v1/security/grants`);
	if (!response.ok) return [];
	const document = (await response.json()) as { grants?: ActiveGrant[] };
	return document.grants ?? [];
}

export async function revokeGrant(grantID: string): Promise<void> {
	const response = await fetch(`${controlPlaneBaseURL}/v1/security/grants/${encodeURIComponent(grantID)}/revoke`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw new Error('Grant revoke failed');
	}
}

export async function readMountedFolders(): Promise<MountedFolder[]> {
	if (!runtimeChild) return [];
	const response = await fetch(`${controlPlaneBaseURL}/v1/filesystem/mounts`);
	if (!response.ok) return [];
	const document = (await response.json()) as { mounts?: MountedFolder[] };
	return document.mounts ?? [];
}

export async function addMountedFolder(path: string): Promise<MountedFolder> {
	const response = await fetch(`${controlPlaneBaseURL}/v1/filesystem/mounts`, {
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
	const response = await fetch(`${controlPlaneBaseURL}/v1/filesystem/mounts/${encodeURIComponent(mountID)}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}

export async function pauseMountedFolder(mountID: string): Promise<void> {
	const response = await fetch(`${controlPlaneBaseURL}/v1/filesystem/mounts/${encodeURIComponent(mountID)}/pause`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}

export async function resumeMountedFolder(mountID: string): Promise<void> {
	const response = await fetch(`${controlPlaneBaseURL}/v1/filesystem/mounts/${encodeURIComponent(mountID)}/resume`, {
		method: 'POST'
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}
