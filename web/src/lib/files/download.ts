import { saveIntoDownloadsFolder, savesIntoDownloadsFolder } from '$lib/native-shell/downloads-folder';
import type { ReportSentBytes } from '$lib/native-shell/file-chunks';
import { downloadNatively, hasNativeDownloading, isFetchableByTheApp } from '$lib/native-shell/file-download';
import { tellKept, tellNotKept } from './download-notice';
import {
	downloadProgress,
	fetchingBeforeHandingOverProgress,
	handingOverAfterFetchingProgress,
	receivingProgress,
	type ReportProgress
} from './download-progress.svelte';
import { fetchedWithProgress } from './fetched-with-progress';
import type { KeptFile } from './kept-file';
import { keepDownloadOnThePhone, keepOnThePhone, keepsFilesOnThePhone } from './phone-keeping';
import { shareThroughTheWebPage, sharesThroughTheWebPage } from './web-share';

const downloadParameter = 'download';
const objectAddressLifetimeMilliseconds = 60_000;

function clickDownloadLink(address: string, filename: string): void {
	const link = document.createElement('a');
	link.href = address;
	link.download = filename;
	link.rel = 'noreferrer';
	link.click();
}

export function downloadAddressOf(source: string, filename: string): string {
	const address = new URL(source);
	if (address.protocol !== 'https:' && address.protocol !== 'http:') return source;
	address.searchParams.set(downloadParameter, filename);
	return address.toString();
}

function keepsFilesInsideTheApp(): boolean {
	return savesIntoDownloadsFolder() || keepsFilesOnThePhone() || sharesThroughTheWebPage();
}

async function keptBlobInsideTheApp(file: Blob, filename: string, onSent: ReportSentBytes): Promise<KeptFile> {
	if (savesIntoDownloadsFolder()) {
		await saveIntoDownloadsFolder(file, filename, onSent);
		return 'downloadsFolder';
	}
	if (keepsFilesOnThePhone()) return keepOnThePhone(file, filename, onSent);
	await shareThroughTheWebPage(file, filename);
	return 'shareSheet';
}

async function keptInsideTheApp(source: string, filename: string, report: ReportProgress): Promise<KeptFile> {
	if (keepsFilesOnThePhone() && hasNativeDownloading() && isFetchableByTheApp(source)) {
		const download = await downloadNatively(source, filename, (receivedBytes, totalBytes) =>
			report(receivingProgress(receivedBytes, totalBytes))
		);
		return keepDownloadOnThePhone(download);
	}
	const file = await fetchedWithProgress(source, (receivedBytes, totalBytes) =>
		report(fetchingBeforeHandingOverProgress(receivedBytes, totalBytes))
	);
	return keptBlobInsideTheApp(file, filename, (sentBytes, totalBytes) =>
		report(handingOverAfterFetchingProgress(sentBytes, totalBytes))
	);
}

async function downloadWithProgress(source: string, filename: string, identity: string): Promise<void> {
	await downloadProgress.run(identity, async (report) => tellKept(await keptInsideTheApp(source, filename, report)));
}

export async function downloadFile(source: string, filename: string, identity: string = source): Promise<void> {
	if (!keepsFilesInsideTheApp()) {
		clickDownloadLink(downloadAddressOf(source, filename), filename);
		return;
	}
	await downloadWithProgress(source, filename, identity);
}

export function startDownload(source: string | Promise<string>, filename: string, identity?: string): void {
	Promise.resolve(source)
		.then((address) => downloadFile(address, filename, identity))
		.catch((failure: unknown) => {
			tellNotKept(failure);
			console.warn('the file was not downloaded', { filename, identity, failure });
		});
}

export async function saveBlob(blob: Blob, filename: string): Promise<void> {
	if (!keepsFilesInsideTheApp()) {
		const address = URL.createObjectURL(blob);
		clickDownloadLink(address, filename);
		setTimeout(() => URL.revokeObjectURL(address), objectAddressLifetimeMilliseconds);
		return;
	}
	tellKept(await keptBlobInsideTheApp(blob, filename, () => {}));
}
