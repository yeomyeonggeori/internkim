import { Command } from '@tauri-apps/plugin-shell';
import type { CompanionStatus, PairingPayload } from './pairing';

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
	await Command.sidecar('binaries/internkim-companion', ['run']).spawn();
}
