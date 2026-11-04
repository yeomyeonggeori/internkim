import { isInsideCapacitorShell, isInsideTauriPhoneShell } from '$lib/native-shell/shell';

export function sharesInsteadOfDownloading(): boolean {
	if (typeof navigator === 'undefined' || typeof navigator.share !== 'function') return false;
	return isInsideTauriPhoneShell() || isInsideCapacitorShell();
}

function wasDismissed(failure: unknown): boolean {
	return failure instanceof DOMException && failure.name === 'AbortError';
}

async function attachmentFile(source: string, filename: string): Promise<File> {
	const answer = await fetch(source);
	if (!answer.ok) throw new Error(`the attachment did not arrive: HTTP ${answer.status}`);
	const arrived = await answer.blob();
	return new File([arrived], filename, { type: arrived.type });
}

export async function shareAttachment(source: string, filename: string): Promise<void> {
	try {
		const file = await attachmentFile(source, filename);
		if (!navigator.canShare?.({ files: [file] })) throw new Error(`this phone cannot share a ${file.type || 'file'} attachment`);
		await navigator.share({ files: [file] });
	} catch (failure) {
		if (wasDismissed(failure)) return;
		console.warn('the attachment was not handed to the share sheet', { source, filename, failure });
	}
}
