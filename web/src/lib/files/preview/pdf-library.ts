import type * as PdfLibrary from 'pdfjs-dist';

let loading: Promise<typeof PdfLibrary> | null = null;

async function importPdfLibrary(): Promise<typeof PdfLibrary> {
	const pdfjs = await import('pdfjs-dist');
	const { default: workerURL } = await import('pdfjs-dist/build/pdf.worker.min.mjs?url');
	pdfjs.GlobalWorkerOptions.workerSrc = workerURL;
	return pdfjs;
}

export function loadPdfLibrary(): Promise<typeof PdfLibrary> {
	loading ??= importPdfLibrary().catch((error: unknown) => {
		loading = null;
		throw error;
	});
	return loading;
}
