import { startDownload } from '$lib/files/download';
import { attachmentSource } from '$lib/stores/attachment-source.svelte';

export function downloadAttachment(shown: string, filename: string, identity: string): void {
	startDownload(attachmentSource.readable(identity, shown), filename, identity);
}
