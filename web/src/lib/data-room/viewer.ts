import { previewKindOf } from '$lib/files/preview/kind';

export type ViewerDocument = {
	id: string;
	title: string;
	fileName: string | null;
	category: string;
	date: string;
	summary: string;
	details: [label: string, value: string][];
};

export type ViewerSource = { url: string; fileName: string; isTextPreview: boolean };

export type FileSigner = (derivedFileName?: string) => Promise<string>;

export const textPreviewFileName = 'content.txt';

export class DataRoomRefused extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
		this.name = 'DataRoomRefused';
	}
}

export function isAccessRefusal(error: unknown): boolean {
	if (!(error instanceof Error) || !('status' in error)) return false;
	return error.status === 400 || error.status === 403 || error.status === 404;
}

export async function previewSourceOf(
	fileName: string,
	signURL: FileSigner,
	mayReadOriginal: boolean
): Promise<ViewerSource | null> {
	if (mayReadOriginal && previewKindOf(fileName) !== 'none') {
		const originalURL = await refusedAsNull(signURL());
		if (originalURL) return { url: originalURL, fileName, isTextPreview: false };
	}
	const textURL = await refusedAsNull(signURL(textPreviewFileName));
	return textURL ? { url: textURL, fileName: textPreviewFileName, isTextPreview: true } : null;
}

async function refusedAsNull(request: Promise<string>): Promise<string | null> {
	return request.catch((error: unknown) => {
		if (isAccessRefusal(error)) return null;
		throw error;
	});
}

const markupEscapes: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };

export function watermarkTile(text: string): string {
	const escaped = text.replace(/[&<>"']/g, (character) => markupEscapes[character]);
	const width = Math.max(320, text.length * 9 + 120);
	const height = Math.round(width * 0.6);
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}"><text x="${width / 2}" y="${height / 2}" text-anchor="middle" dominant-baseline="middle" transform="rotate(-30 ${width / 2} ${height / 2})" font-family="system-ui, sans-serif" font-size="15" fill="rgba(128,128,128,0.28)">${escaped}</text></svg>`;
	return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
}

export function neighbourID(
	documents: ViewerDocument[],
	openID: string,
	step: -1 | 1
): string | undefined {
	const index = documents.findIndex((document) => document.id === openID);
	if (index < 0) return undefined;
	return documents[index + step]?.id;
}
