import { invoke } from '@tauri-apps/api/core';
import type { Child } from '@tauri-apps/plugin-shell';
import { Command } from '@tauri-apps/plugin-shell';
import type { CompanionStatus, PairingPayload } from './pairing';

export type RuntimeStatus = {
	isRunning: boolean;
	processID?: number;
	lastError?: string;
};

let runtimeChild: Child | undefined;
let runtimeStatus: RuntimeStatus = { isRunning: false };

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
	const shellBridgeURL = await invoke<string>('start_shell_bridge');
	const command = Command.sidecar('binaries/internkim-companion', ['run', '--shell-bridge-url', shellBridgeURL]);
	command.on('close', () => {
		runtimeChild = undefined;
		runtimeStatus = { isRunning: false };
	});
	command.on('error', (errorValue) => {
		runtimeChild = undefined;
		runtimeStatus = { isRunning: false, lastError: errorValue };
	});
	runtimeChild = await command.spawn();
	runtimeStatus = { isRunning: true, processID: runtimeChild.pid };
}

export function readRuntimeStatus(): RuntimeStatus {
	return runtimeStatus;
}
