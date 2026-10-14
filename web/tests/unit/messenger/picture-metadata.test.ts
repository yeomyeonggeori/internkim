import { describe, expect, test } from 'bun:test';
import { withoutPictureMetadata } from '../../../src/lib/components/channel/picture-metadata';

function jpegSegment(marker: number, payload: number[]): number[] {
	const length = payload.length + 2;
	return [0xff, marker, length >> 8, length & 0xff, ...payload];
}

const ascii = (text: string): number[] => [...new TextEncoder().encode(text)];
const startOfImage = [0xff, 0xd8];
const canonicalJfif = jpegSegment(0xe0, [...ascii('JFIF\0'), 1, 1, 0, 0, 1, 0, 1, 0, 0]);
const colourProfile = jpegSegment(0xe2, [...ascii('ICC_PROFILE\0'), 1, 1, 9, 9, 9]);
const exif = jpegSegment(0xe1, [...ascii('Exif\0\0'), 1, 2, 3]);
const comment = jpegSegment(0xfe, ascii('made by a camera'));
const quantisationTable = jpegSegment(0xdb, [0, 1, 2, 3]);
const scanAndImageData = [...jpegSegment(0xda, [1, 2, 3]), 0x12, 0xff, 0x00, 0x34, 0xff, 0xd9];

function pngChunk(type: string, data: number[]): number[] {
	const length = data.length;
	return [length >>> 24, (length >> 16) & 0xff, (length >> 8) & 0xff, length & 0xff, ...ascii(type), ...data, 0, 0, 0, 0];
}

const pngSignature = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
const header = pngChunk('IHDR', [0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0]);
const pixels = pngChunk('IDAT', [1, 2, 3]);
const end = pngChunk('IEND', []);

describe('what a picture carries to the messenger', () => {
	test('a canvas JPEG keeps its frame and pixels and loses its colour profile', () => {
		const encoded = new Uint8Array([...startOfImage, ...canonicalJfif, ...colourProfile, ...quantisationTable, ...scanAndImageData]);
		expect([...withoutPictureMetadata(encoded, 'image/jpeg')]).toEqual([
			...startOfImage,
			...canonicalJfif,
			...quantisationTable,
			...scanAndImageData
		]);
	});

	test('camera details and comments are left behind', () => {
		const encoded = new Uint8Array([...startOfImage, ...exif, ...comment, ...quantisationTable, ...scanAndImageData]);
		expect([...withoutPictureMetadata(encoded, 'image/jpeg')]).toEqual([...startOfImage, ...quantisationTable, ...scanAndImageData]);
	});

	test('a PNG loses its colour profile and text and keeps its image chunks', () => {
		const encoded = new Uint8Array([
			...pngSignature,
			...header,
			...pngChunk('iCCP', [1, 2]),
			...pngChunk('tEXt', ascii('Software\0canvas')),
			...pixels,
			...end
		]);
		expect([...withoutPictureMetadata(encoded, 'image/png')]).toEqual([...pngSignature, ...header, ...pixels, ...end]);
	});

	test('bytes that are not the named picture are passed through untouched', () => {
		const notAJpeg = new Uint8Array([1, 2, 3, 4, 5]);
		expect(withoutPictureMetadata(notAJpeg, 'image/jpeg')).toBe(notAJpeg);
	});
});
