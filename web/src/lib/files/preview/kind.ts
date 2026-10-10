export type PreviewKind = 'pdf' | 'image' | 'spreadsheet' | 'text' | 'none';

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
	txt: 'text',
	md: 'text',
	markdown: 'text',
	log: 'text',
	json: 'text'
};

export function extensionOf(fileName: string): string {
	const extensionAt = fileName.lastIndexOf('.');
	return extensionAt > 0 ? fileName.slice(extensionAt + 1).toLowerCase() : '';
}

export function previewKindOf(fileName: string): PreviewKind {
	return kindsByExtension[extensionOf(fileName)] ?? 'none';
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
