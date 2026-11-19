export type ReportFetchedBytes = (receivedBytes: number, totalBytes: number) => void;

function announcedSizeOf(answer: Response): number {
	const announced = Number(answer.headers.get('content-length'));
	return Number.isFinite(announced) && announced > 0 ? announced : 0;
}

export async function fetchedWithProgress(source: string, onFetched: ReportFetchedBytes): Promise<Blob> {
	const answer = await fetch(source);
	if (!answer.ok) throw new Error(`the file did not arrive: HTTP ${answer.status}`);
	if (!answer.body) return answer.blob();
	const totalBytes = announcedSizeOf(answer);
	const reader = answer.body.getReader();
	const parts: Uint8Array<ArrayBuffer>[] = [];
	let receivedBytes = 0;
	for (let read = await reader.read(); !read.done; read = await reader.read()) {
		parts.push(read.value);
		receivedBytes += read.value.byteLength;
		onFetched(receivedBytes, totalBytes);
	}
	return new Blob(parts, { type: answer.headers.get('content-type') ?? '' });
}
