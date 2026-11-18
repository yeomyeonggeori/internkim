import { isInsideTauriPhoneShell } from '$lib/native-shell/shell';

export function sharesThroughTheWebPage(): boolean {
	if (typeof navigator === 'undefined' || typeof navigator.share !== 'function') return false;
	return isInsideTauriPhoneShell();
}

function wasDismissed(failure: unknown): boolean {
	return failure instanceof DOMException && failure.name === 'AbortError';
}

export async function shareThroughTheWebPage(file: Blob, filename: string): Promise<void> {
	const shared = new File([file], filename, { type: file.type });
	if (!navigator.canShare?.({ files: [shared] })) throw new Error(`this phone cannot share a ${shared.type || 'file'} file`);
	try {
		await navigator.share({ files: [shared] });
	} catch (failure) {
		if (wasDismissed(failure)) return;
		throw failure;
	}
}
