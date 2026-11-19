import { toast } from 'svelte-sonner';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { downloadText } from '$lib/i18n/download-text';
import { wasRefusedPhotoPermission } from '$lib/native-shell/media-save';
import { shellPlatform } from '$lib/native-shell/shell';
import type { KeptFile } from './kept-file';

const text = createPageText(downloadText);

function savedIntoPhotosText(): string {
	return shellPlatform() === 'ios' ? text.savedToPhotoLibrary : text.savedToGallery;
}

export function tellKept(kept: KeptFile): void {
	if (kept === 'photoLibrary') toast.success(savedIntoPhotosText());
	else if (kept === 'downloadsFolder') toast.success(text.saved);
}

export function tellNotKept(failure: unknown): void {
	toast.error(wasRefusedPhotoPermission(failure) ? text.notSavedWithoutPhotoPermission : text.notSaved);
}
