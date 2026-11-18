import { sendFileInChunks } from './file-chunks';

export const shareSheetPresenceMarker = '__INTERNKIM_SHARE_SHEET__';

export function hasNativeShareSheet(): boolean {
	return typeof window !== 'undefined' && shareSheetPresenceMarker in window;
}

export async function presentShareSheet(file: Blob, filename: string): Promise<void> {
	await sendFileInChunks('plugin:share-sheet|present', file, filename);
}
