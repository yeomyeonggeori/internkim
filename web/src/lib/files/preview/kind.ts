import { codeLanguageForFile } from '$lib/files/view';

export type PreviewKind =
	| 'pdf'
	| 'image'
	| 'spreadsheet'
	| 'presentation'
	| 'word'
	| 'html'
	| 'markdown'
	| 'code'
	| 'text'
	| 'none';

const kindsByExtension: Record<string, PreviewKind> = {
	pdf: 'pdf',
	png: 'image',
	jpg: 'image',
	jpeg: 'image',
	gif: 'image',
	webp: 'image',
	svg: 'image',
	bmp: 'image',
	avif: 'image',
	xlsx: 'spreadsheet',
	xlsm: 'spreadsheet',
	xls: 'spreadsheet',
	ods: 'spreadsheet',
	csv: 'spreadsheet',
	tsv: 'spreadsheet',
	pptx: 'presentation',
	docx: 'word',
	html: 'html',
	htm: 'html',
	md: 'markdown',
	markdown: 'markdown',
	txt: 'text',
	text: 'text',
	log: 'text'
};

export function extensionOf(fileName: string): string {
	const extensionAt = fileName.lastIndexOf('.');
	return extensionAt > 0 ? fileName.slice(extensionAt + 1).toLowerCase() : '';
}

export function previewKindOf(fileName: string): PreviewKind {
	const kind = kindsByExtension[extensionOf(fileName)];
	if (kind) return kind;
	return codeLanguageForFile(fileName) ? 'code' : 'none';
}

const textualKinds: PreviewKind[] = ['html', 'markdown', 'code', 'text'];
const maxTextualPreviewBytes = 2 * 1024 * 1024;
const maxBinaryPreviewBytes = 50 * 1024 * 1024;

export function isTooLargeToPreview(fileName: string, size: number): boolean {
	const limit = textualKinds.includes(previewKindOf(fileName)) ? maxTextualPreviewBytes : maxBinaryPreviewBytes;
	return size > limit;
}

export type CardPreview = 'thumbnail' | 'document' | 'none';

const maxCardPreviewBytes = 20 * 1024 * 1024;

export function cardPreviewOf(fileName: string, sizeBytes?: number): CardPreview {
	const kind = previewKindOf(fileName);
	if (kind === 'none') return 'none';
	if (sizeBytes !== undefined && (sizeBytes > maxCardPreviewBytes || isTooLargeToPreview(fileName, sizeBytes))) return 'none';
	return kind === 'pdf' || kind === 'spreadsheet' ? 'thumbnail' : 'document';
}

const tallestCardAspectRatio = Math.SQRT1_2;
const widestCardAspectRatio = 3;

export function cardAspectRatioOf(documentAspectRatio: number): number {
	return Math.min(widestCardAspectRatio, Math.max(tallestCardAspectRatio, documentAspectRatio));
}

export type SheetGrid = { rows: string[][]; columnCount: number; isTruncated: boolean };

export const maxPreviewRows = 1000;
export const maxPreviewColumns = 60;

export function boundedGrid(rows: unknown[][]): SheetGrid {
	const isTruncated =
		rows.length > maxPreviewRows || rows.some((row) => row.length > maxPreviewColumns);
	const visibleRows = rows
		.slice(0, maxPreviewRows)
		.map((row) => Array.from(row.slice(0, maxPreviewColumns), (cell) => (cell == null ? '' : String(cell))));
	const columnCount = Math.max(0, ...visibleRows.map((row) => row.length));
	return { rows: visibleRows, columnCount, isTruncated };
}

export function columnLabel(index: number): string {
	const letter = String.fromCharCode(65 + (index % 26));
	return index < 26 ? letter : columnLabel(Math.floor(index / 26) - 1) + letter;
}
