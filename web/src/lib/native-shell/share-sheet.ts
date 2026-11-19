import { sendFileInChunks, type ReportSentBytes } from './file-chunks';
import type { NativeDownload } from './file-download';

export const shareSheetPresenceMarker = '__INTERNKIM_SHARE_SHEET__';

export function hasNativeShareSheet(): boolean {
	return typeof window !== 'undefined' && shareSheetPresenceMarker in window;
}

export async function presentShareSheet(file: Blob, filename: string, onSent: ReportSentBytes): Promise<void> {
	await sendFileInChunks('plugin:share-sheet|present', file, filename, onSent);
}

export async function presentDownloadInShareSheet(download: NativeDownload): Promise<void> {
	const { invoke } = await import('@tauri-apps/api/core');
	await invoke<void>('plugin:share-sheet|present_download', { transfer: download.transfer, mediaType: download.mediaType });
}
