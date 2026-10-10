import { degrees, PDFDocument, rgb, StandardFonts } from 'pdf-lib';

const watermarkGray = rgb(0.45, 0.45, 0.45);
const diagonalOpacity = 0.14;
const footerOpacity = 0.6;
const footerSize = 7;

export function winAnsiText(text: string): string {
	return text.replace(/[^\x20-\x7E]/g, '?');
}

export async function watermarkedPDF(original: Uint8Array, lines: string[]): Promise<Uint8Array> {
	const document = await PDFDocument.load(original);
	const font = await document.embedFont(StandardFonts.Helvetica);
	const safeLines = lines.map(winAnsiText);
	for (const page of document.getPages()) {
		const { width, height } = page.getSize();
		const size = Math.max(12, Math.min(width, height) / 22);
		const lineHeight = size * 1.4;
		const rows = [height * 0.25, height * 0.55, height * 0.85];
		for (const rowY of rows) {
			safeLines.forEach((line, index) => {
				const lineWidth = font.widthOfTextAtSize(line, size);
				page.drawText(line, {
					x: width / 2 - (lineWidth / 2) * Math.SQRT1_2,
					y: rowY - (lineWidth / 2) * Math.SQRT1_2 - index * lineHeight,
					size,
					font,
					color: watermarkGray,
					opacity: diagonalOpacity,
					rotate: degrees(45)
				});
			});
		}
		page.drawText(safeLines.join('  |  '), {
			x: 24,
			y: 12,
			size: footerSize,
			font,
			color: watermarkGray,
			opacity: footerOpacity
		});
	}
	return document.save();
}
