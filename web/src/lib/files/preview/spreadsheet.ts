import { boundedGrid, extensionOf, type SheetGrid } from './kind';

export type Sheet = { name: string; grid: SheetGrid };

const delimitedExtensions = ['csv', 'tsv'];

export async function readSheets(bytes: ArrayBuffer, fileName: string, rowLimit: number): Promise<Sheet[]> {
	const spreadsheet = await import('xlsx');
	const options = { dense: true, cellDates: true, sheetRows: rowLimit + 1 } as const;
	const workbook = delimitedExtensions.includes(extensionOf(fileName))
		? spreadsheet.read(new TextDecoder().decode(bytes), { ...options, type: 'string' })
		: spreadsheet.read(bytes, { ...options, type: 'array' });
	return workbook.SheetNames.map((name) => ({
		name,
		grid: boundedGrid(
			spreadsheet.utils.sheet_to_json<unknown[]>(workbook.Sheets[name], { header: 1, raw: false, defval: '', blankrows: true })
		)
	}));
}
