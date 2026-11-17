import { isInsideTauriPhoneShell } from '$lib/native-shell/shell';

const downloadParameter = 'download';
const objectAddressLifetimeMilliseconds = 60_000;

function sharesInsteadOfDownloading(): boolean {
	if (typeof navigator === 'undefined' || typeof navigator.share !== 'function') return false;
	return isInsideTauriPhoneShell();
}

function wasDismissed(failure: unknown): boolean {
	return failure instanceof DOMException && failure.name === 'AbortError';
}

async function shareFile(file: File): Promise<void> {
	if (!navigator.canShare?.({ files: [file] })) throw new Error(`this phone cannot share a ${file.type || 'file'} file`);
	try {
		await navigator.share({ files: [file] });
	} catch (failure) {
		if (wasDismissed(failure)) return;
		throw failure;
	}
}

async function fetchedFile(source: string, filename: string): Promise<File> {
	const answer = await fetch(source);
	if (!answer.ok) throw new Error(`the file did not arrive: HTTP ${answer.status}`);
	const arrived = await answer.blob();
	return new File([arrived], filename, { type: arrived.type });
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
	if (sharesInsteadOfDownloading()) {
		await shareFile(await fetchedFile(source, filename));
		return;
	}
	clickDownloadLink(downloadAddressOf(source, filename), filename);
}

export function startDownload(source: string, filename: string): void {
	downloadFile(source, filename).catch((failure: unknown) =>
		console.warn('the file was not handed to the share sheet', { source, filename, failure })
	);
}

export async function saveBlob(blob: Blob, filename: string): Promise<void> {
	if (sharesInsteadOfDownloading()) {
		await shareFile(new File([blob], filename, { type: blob.type }));
		return;
	}
	const address = URL.createObjectURL(blob);
	clickDownloadLink(address, filename);
	setTimeout(() => URL.revokeObjectURL(address), objectAddressLifetimeMilliseconds);
}
