import { invoke } from '@tauri-apps/api/core';
import type { Child } from '@tauri-apps/plugin-shell';
import { Command } from '@tauri-apps/plugin-shell';
import type { CompanionStatus, PairingPayload } from './pairing';
import { loadCompanionSettings, settingsToSidecarArguments } from './settings';

export type RuntimeStatus = {
	isRunning: boolean;
	processID?: number;
	lastError?: string;
	lastHeartbeatAt?: string;
	restartAttempts?: number;
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
		const document = (await response.json()) as { lastHeartbeatAt?: string; lastError?: string };
		runtimeStatus = {
			...runtimeStatus,
			lastHeartbeatAt: document.lastHeartbeatAt,
			lastError: document.lastError || runtimeStatus.lastError
		};
		return runtimeStatus;
	} catch {
		return runtimeStatus;
	}
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
