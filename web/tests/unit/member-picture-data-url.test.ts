import { describe, expect, test } from 'bun:test';
import { blobOfDataURL } from '../../src/lib/profile/keep-member-picture';

describe('the messenger picture kept as a member picture', () => {
	test('keeps the bytes and the type the messenger answered', async () => {
		const blob = blobOfDataURL('data:image/jpeg;base64,/9j/4AA=');
		expect(blob.type).toBe('image/jpeg');
		expect([...new Uint8Array(await blob.arrayBuffer())]).toEqual([255, 216, 255, 224, 0]);
	});

	test('takes a picture a phone saved as heic', () => {
		expect(blobOfDataURL('data:image/heic;base64,AAAA').type).toBe('image/heic');
	});

	test('refuses a picture it cannot keep, rather than passing for someone with no picture', () => {
		expect(() => blobOfDataURL('data:image/svg+xml;base64,PHN2Zy8+')).toThrow('image/svg+xml');
		expect(() => blobOfDataURL('https://example.com/picture.png')).toThrow('data url');
	});
});
