import { describe, expect, test } from 'bun:test';
import { extensionOfBytes } from '../../../src/lib/server/data-room-legacy-originals';

const encoded = (text: string) => new TextEncoder().encode(text);

describe('the extension of a legacy original', () => {
	test('is read from a binary signature', () => {
		expect(extensionOfBytes(encoded('%PDF-1.7\n'))).toBe('pdf');
		expect(extensionOfBytes(new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a]))).toBe('png');
		expect(extensionOfBytes(new Uint8Array([0xff, 0xd8, 0xff, 0xe0]))).toBe('jpg');
	});

	test('names an Office archive by the folder it carries', () => {
		const archive = (folder: string) => new Uint8Array([0x50, 0x4b, 0x03, 0x04, ...encoded(`[Content_Types].xml${folder}slide1.xml`)]);
		expect(extensionOfBytes(archive('ppt/'))).toBe('pptx');
		expect(extensionOfBytes(archive('word/'))).toBe('docx');
		expect(extensionOfBytes(archive('xl/'))).toBe('xlsx');
		expect(extensionOfBytes(archive('other/'))).toBe('zip');
	});

	test('tells a page, a drawing, markdown and plain text apart', () => {
		expect(extensionOfBytes(encoded('\n<!DOCTYPE html><html></html>'))).toBe('html');
		expect(extensionOfBytes(encoded('<svg xmlns="http://www.w3.org/2000/svg"/>'))).toBe('svg');
		expect(extensionOfBytes(encoded('# 서비스 개요\n'))).toBe('md');
		expect(extensionOfBytes(encoded('---\ntitle: 서비스 개요\n---\n\n본문'))).toBe('md');
		expect(extensionOfBytes(encoded('plain notes'))).toBe('txt');
	});
});
