const downloadParameter = 'download';

export function downloadAddressOf(source: string, filename: string): string {
	const address = new URL(source);
	if (address.protocol !== 'https:' && address.protocol !== 'http:') return source;
	address.searchParams.set(downloadParameter, filename);
	return address.toString();
}

export function startDownload(source: string, filename: string): void {
	const link = document.createElement('a');
	link.href = downloadAddressOf(source, filename);
	link.download = filename;
	link.rel = 'noreferrer';
	link.click();
}
