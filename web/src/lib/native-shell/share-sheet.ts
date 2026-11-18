import { sendFileInChunks } from './file-chunks';

export async function presentShareSheet(file: Blob, filename: string): Promise<void> {
	await sendFileInChunks('plugin:share-sheet|present', file, filename);
}
