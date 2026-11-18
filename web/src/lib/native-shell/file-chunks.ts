const chunkBytes = 2 * 1024 * 1024;
const unnamedMediaType = 'application/octet-stream';
const transferIdentifierBytes = 16;

function newTransferIdentifier(): string {
	const randomBytes = crypto.getRandomValues(new Uint8Array(transferIdentifierBytes));
	return Array.from(randomBytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
}

export async function sendFileInChunks(command: string, file: Blob, filename: string): Promise<void> {
	const { invoke } = await import('@tauri-apps/api/core');
	const transfer = newTransferIdentifier();
	const mediaType = encodeURIComponent(file.type || unnamedMediaType);
	const chunkCount = Math.max(1, Math.ceil(file.size / chunkBytes));
	for (let index = 0; index < chunkCount; index += 1) {
		const start = index * chunkBytes;
		const content = new Uint8Array(await file.slice(start, start + chunkBytes).arrayBuffer());
		await invoke<void>(command, content, {
			headers: {
				transfer,
				filename: encodeURIComponent(filename),
				'media-type': mediaType,
				chunk: String(index),
				'last-chunk': String(index === chunkCount - 1)
			}
		});
	}
}
