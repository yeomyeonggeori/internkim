import { sendFileInChunks, type ReportSentBytes } from './file-chunks';
import { isInsideTauriDesktopShell } from './shell';

export function savesIntoDownloadsFolder(): boolean {
	return isInsideTauriDesktopShell();
}

export async function saveIntoDownloadsFolder(content: Blob, filename: string, onSent: ReportSentBytes): Promise<void> {
	await sendFileInChunks('plugin:file-save|save', content, filename, onSent);
}
