import { isInsideTauriShell, shellPlatform } from './shell';

export function printsThroughTheSystemSheet(): boolean {
	return isInsideTauriShell() && shellPlatform() === 'ios';
}

export async function presentPrintSheet(title: string, html: string): Promise<void> {
	const { invoke } = await import('@tauri-apps/api/core');
	await invoke<void>('plugin:print-sheet|present', { title, html });
}
