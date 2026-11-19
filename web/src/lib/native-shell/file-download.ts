import { z } from 'zod';
import { newTransferIdentifier } from './transfer-identifier';

export const fileDownloadPresenceMarker = '__INTERNKIM_FILE_DOWNLOAD__';
const fetchableProtocols = ['https:', 'http:'];

const progressMessage = z.object({ receivedBytes: z.number(), totalBytes: z.number() });
const downloadedAnswer = z.object({ mediaType: z.string() });

export type NativeDownload = { transfer: string; mediaType: string };

export type ReportReceivedBytes = (receivedBytes: number, totalBytes: number) => void;

export function hasNativeDownloading(): boolean {
	return typeof window !== 'undefined' && fileDownloadPresenceMarker in window;
}

export function isFetchableByTheApp(source: string): boolean {
	if (!URL.canParse(source)) return false;
	return fetchableProtocols.includes(new URL(source).protocol);
}

export async function downloadNatively(address: string, filename: string, onReceived: ReportReceivedBytes): Promise<NativeDownload> {
	const { Channel, invoke } = await import('@tauri-apps/api/core');
	const transfer = newTransferIdentifier();
	const onProgress = new Channel<unknown>((message) => {
		const progress = progressMessage.safeParse(message);
		if (progress.success) onReceived(progress.data.receivedBytes, progress.data.totalBytes);
		else console.warn('the app reported download progress the page does not read', { message, issues: progress.error.issues });
	});
	const answer = await invoke<unknown>('plugin:file-download|download', { address, filename, transfer, onProgress });
	return { transfer, mediaType: downloadedAnswer.parse(answer).mediaType };
}
