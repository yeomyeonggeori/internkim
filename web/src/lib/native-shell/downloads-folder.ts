import { isInsideTauriDesktopShell } from './shell';

export function savesIntoDownloadsFolder(): boolean {
	return isInsideTauriDesktopShell();
}

export async function saveIntoDownloadsFolder(content: Blob, filename: string): Promise<void> {
	const { invoke } = await import('@tauri-apps/api/core');
	await invoke<void>('plugin:file-save|save', new Uint8Array(await content.arrayBuffer()), {
		headers: { filename: encodeURIComponent(filename) }
	});
}
