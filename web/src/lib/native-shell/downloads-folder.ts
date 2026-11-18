import { sendFileInChunks } from './file-chunks';
import { isInsideTauriDesktopShell } from './shell';

export function savesIntoDownloadsFolder(): boolean {
	return isInsideTauriDesktopShell();
}

export async function saveIntoDownloadsFolder(content: Blob, filename: string): Promise<void> {
	await sendFileInChunks('plugin:file-save|save', content, filename);
}
