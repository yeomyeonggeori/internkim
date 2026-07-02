import { invoke } from '@tauri-apps/api/core';

export type BackendEndpoint = {
	baseURL: string;
	model: string;
	embeddingModel?: string;
};

export type CompanionSettings = {
	schemaVersion: number;
	preferCompanionBrowser: boolean;
	enableLocalLLM: boolean;
	localBackendOrder: string[];
	ollama: BackendEndpoint;
	llamacpp: BackendEndpoint;
	mlx: BackendEndpoint;
	stt: unknown | null;
	tts: unknown | null;
};

export const defaultSettings: CompanionSettings = {
	schemaVersion: 1,
	preferCompanionBrowser: false,
	enableLocalLLM: false,
	localBackendOrder: ['ollama'],
	ollama: { baseURL: 'http://127.0.0.1:11434', model: '' },
	llamacpp: { baseURL: 'http://127.0.0.1:8080', model: '', embeddingModel: 'embeddinggemma' },
	mlx: { baseURL: 'http://127.0.0.1:10240', model: '' },
	stt: null,
	tts: null
};

export async function loadCompanionSettings(): Promise<CompanionSettings> {
	try {
		return await invoke<CompanionSettings>('get_settings');
	} catch (error) {
		console.error('failed to load companion settings', error);
		return defaultSettings;
	}
}

export async function saveCompanionSettings(
	settings: CompanionSettings
): Promise<CompanionSettings> {
	return await invoke<CompanionSettings>('set_settings', { settings });
}

export async function fetchBackendModels(
	backendName: string,
	baseURL: string
): Promise<string[]> {
	const trimmedBaseURL = baseURL.trim().replace(/\/+$/, '');
	if (!trimmedBaseURL) return [];
	const url =
		backendName === 'ollama'
			? `${trimmedBaseURL}/api/tags`
			: `${trimmedBaseURL}/v1/models`;
	try {
		const response = await fetch(url);
		if (!response.ok) return [];
		const document = (await response.json()) as Record<string, unknown>;
		if (backendName === 'ollama') {
			const models = document.models as Array<{ name?: string }> | undefined;
			return models?.map((entry) => entry.name ?? '').filter(Boolean) ?? [];
		}
		const data = document.data as Array<{ id?: string }> | undefined;
		return data?.map((entry) => entry.id ?? '').filter(Boolean) ?? [];
	} catch {
		return [];
	}
}
