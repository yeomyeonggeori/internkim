import { saveIntoDownloadsFolder, savesIntoDownloadsFolder } from '$lib/native-shell/downloads-folder';
import { keepOnThePhone, keepsFilesOnThePhone } from './phone-keeping';
import { shareThroughTheWebPage, sharesThroughTheWebPage } from './web-share';

const downloadParameter = 'download';
const objectAddressLifetimeMilliseconds = 60_000;

async function fetchedBlob(source: string): Promise<Blob> {
	const answer = await fetch(source);
	if (!answer.ok) throw new Error(`the file did not arrive: HTTP ${answer.status}`);
	return answer.blob();
}

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

export async function downloadFile(source: string, filename: string): Promise<void> {
	if (savesIntoDownloadsFolder()) {
		await saveIntoDownloadsFolder(await fetchedBlob(source), filename);
		return;
	}
	if (keepsFilesOnThePhone()) {
		await keepOnThePhone(await fetchedBlob(source), filename);
		return;
	}
	if (sharesThroughTheWebPage()) {
		await shareThroughTheWebPage(await fetchedBlob(source), filename);
		return;
	}
	clickDownloadLink(downloadAddressOf(source, filename), filename);
}

export function startDownload(source: string, filename: string): void {
	downloadFile(source, filename).catch((failure: unknown) =>
		console.warn('the file was not downloaded', { source, filename, failure })
	);
}

export async function saveBlob(blob: Blob, filename: string): Promise<void> {
	if (savesIntoDownloadsFolder()) {
		await saveIntoDownloadsFolder(blob, filename);
		return;
	}
	if (keepsFilesOnThePhone()) {
		await keepOnThePhone(blob, filename);
		return;
	}
	if (sharesThroughTheWebPage()) {
		await shareThroughTheWebPage(blob, filename);
		return;
	}
	const address = URL.createObjectURL(blob);
	clickDownloadLink(address, filename);
	setTimeout(() => URL.revokeObjectURL(address), objectAddressLifetimeMilliseconds);
}
