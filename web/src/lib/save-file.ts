import { saveBlob } from '$lib/files/download';

export function saveFile(filename: string, content: string, contentType: string): void {
	saveBlob(new Blob([content], { type: contentType }), filename).catch((failure: unknown) =>
		console.warn('the file was not handed to the share sheet', { filename, failure })
	);
}
