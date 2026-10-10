import { describe, expect, test } from 'bun:test';
import { PDFDocument } from 'pdf-lib';
import { getDocument } from 'pdfjs-dist/legacy/build/pdf.mjs';
import { watermarkedPDF, winAnsiText } from '../../../src/lib/server/pdf-watermark';
import { attachmentHeader, downloadedAt } from '../../../src/lib/server/data-room-download';

async function samplePDF(pageCount: number): Promise<Uint8Array> {
	const document = await PDFDocument.create();
	for (let index = 0; index < pageCount; index += 1) document.addPage([595, 842]);
	return document.save();
}

async function textOfEachPage(bytes: Uint8Array): Promise<string[]> {
	const loading = getDocument({ data: bytes.slice(), useWorkerFetch: false });
	const opened = await loading.promise;
	const pages: string[] = [];
	for (let number = 1; number <= opened.numPages; number += 1) {
		const content = await (await opened.getPage(number)).getTextContent();
		pages.push(content.items.map((item) => ('str' in item ? item.str : '')).join(' '));
	}
	await loading.destroy();
	return pages;
}

describe('watermarkedPDF', () => {
	test('stamps who downloaded it and when on every page', async () => {
		const stamped = await watermarkedPDF(await samplePDF(3), ['reviewer@example.com', '2026-10-10 13:45 UTC']);
		const pages = await textOfEachPage(stamped);
		expect(pages).toHaveLength(3);
		for (const page of pages) {
			expect(page).toContain('reviewer@example.com');
			expect(page).toContain('2026-10-10 13:45 UTC');
		}
	});

	test('refuses a document it cannot read rather than handing it out unstamped', async () => {
		const unreadable = new TextEncoder().encode('%PDF-1.7\n1 0 obj\n<< /Encrypt 2 0 R >>\nendobj\ntrailer\n<< /Root 1 0 R /Encrypt 2 0 R >>\n%%EOF');
		await expect(watermarkedPDF(unreadable, ['x'])).rejects.toThrow();
	});
});

describe('winAnsiText', () => {
	test('keeps printable ASCII and replaces what a standard font cannot draw', () => {
		expect(winAnsiText('link 7acbb02c · 투자사')).toBe('link 7acbb02c ? ???');
	});
});

describe('attachmentHeader', () => {
	test('names the file for every browser, keeping its Korean title where the browser can read it', () => {
		expect(attachmentHeader('2025 감사보고서.pdf')).toBe(
			`attachment; filename="2025 _____.pdf"; filename*=UTF-8''2025%20%EA%B0%90%EC%82%AC%EB%B3%B4%EA%B3%A0%EC%84%9C.pdf`
		);
		expect(attachmentHeader('a"b.pdf')).toContain('filename="a_b.pdf"');
	});
});

describe('downloadedAt', () => {
	test('writes the moment in UTC to the minute', () => {
		expect(downloadedAt(new Date('2026-10-10T13:45:59Z'))).toBe('2026-10-10 13:45 UTC');
	});
});
