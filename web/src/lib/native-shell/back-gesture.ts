import { isInsideTauriShell, shellPlatform } from './shell';

export async function allowBackGesture(isAllowed: boolean): Promise<void> {
	if (!isInsideTauriShell() || shellPlatform() !== 'ios') return;
	const { invoke } = await import('@tauri-apps/api/core');
	await invoke<void>('plugin:back-gesture|apply', { isAllowed });
}
