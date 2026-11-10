import { previewKindOf, type SheetGrid } from './kind';
import { loadPdfLibrary } from './pdf-library';
import { readSheets } from './spreadsheet';

export type Thumbnail =
	| { kind: 'page'; address: string; pageCount: number; aspectRatio: number }
	| { kind: 'sheet'; grid: SheetGrid; sheetCount: number };

const thumbnailWidthPixels = 416;
export const usualPageAspectRatio = Math.SQRT1_2;
const sheetRows = 14;
const sheetColumns = 6;
const maxThumbnailBytes = 20 * 1024 * 1024;
const thumbnailsByURL = new Map<string, Promise<Thumbnail | null>>();

export function hasThumbnail(fileName: string, sizeBytes?: number): boolean {
	if (sizeBytes !== undefined && sizeBytes > maxThumbnailBytes) return false;
	const kind = previewKindOf(fileName);
	return kind === 'pdf' || kind === 'spreadsheet';
}

export function thumbnailOf(url: string, readAddress: () => string, fileName: string): Promise<Thumbnail | null> {
	const known = thumbnailsByURL.get(url);
	if (known) return known;
	const drawing = fetchBytes(readAddress())
		.then((bytes) => drawThumbnail(bytes, fileName))
		.catch((error: unknown) => {
			thumbnailsByURL.delete(url);
			throw error;
		});
	thumbnailsByURL.set(url, drawing);
	return drawing;
}

async function fetchBytes(address: string): Promise<ArrayBuffer> {
	const response = await fetch(address);
	if (!response.ok) throw new Error(`the file answered ${response.status}`);
	return response.arrayBuffer();
}

function drawThumbnail(bytes: ArrayBuffer, fileName: string): Promise<Thumbnail | null> {
	const kind = previewKindOf(fileName);
	if (kind === 'pdf') return firstPageOf(bytes);
	if (kind === 'spreadsheet') return firstRowsOf(bytes, fileName);
	return Promise.resolve(null);
}

async function firstPageOf(bytes: ArrayBuffer): Promise<Thumbnail> {
	const pdfjs = await loadPdfLibrary();
	const loadingTask = pdfjs.getDocument({ data: new Uint8Array(bytes) });
	try {
		const pdfDocument = await loadingTask.promise;
		const page = await pdfDocument.getPage(1);
		const viewport = page.getViewport({ scale: thumbnailWidthPixels / page.getViewport({ scale: 1 }).width });
		const canvas = document.createElement('canvas');
		canvas.width = Math.floor(viewport.width);
		canvas.height = Math.floor(viewport.height);
		await page.render({ canvas, viewport }).promise;
		const aspectRatio = viewport.width / viewport.height;
		return { kind: 'page', address: await objectAddressOf(canvas), pageCount: pdfDocument.numPages, aspectRatio };
	} finally {
		void loadingTask.destroy();
	}
}

function objectAddressOf(canvas: HTMLCanvasElement): Promise<string> {
	return new Promise((resolve, reject) =>
		canvas.toBlob((blob) => (blob ? resolve(URL.createObjectURL(blob)) : reject(new Error('the page did not draw'))), 'image/png')
	);
}

async function firstRowsOf(bytes: ArrayBuffer, fileName: string): Promise<Thumbnail> {
	const sheets = await readSheets(bytes, fileName, sheetRows);
	const first = sheets[0]?.grid ?? { rows: [], columnCount: 0, isTruncated: false };
	const rows = Array.from({ length: sheetRows }, (_, rowIndex) =>
		Array.from({ length: sheetColumns }, (_, columnIndex) => first.rows[rowIndex]?.[columnIndex] ?? '')
	);
	return {
		kind: 'sheet',
		grid: { rows, columnCount: sheetColumns, isTruncated: first.isTruncated },
		sheetCount: sheets.length
	};
}
