import { describe, expect, test } from 'bun:test';
import { isHeicPicture, pictureEveryBrowserShows, pictureFileEveryBrowserShows } from '../../src/lib/files/heic-picture';

describe('a picture only some browsers can show', () => {
	test('is one a phone saved as heic or heif, read past any charset the browser adds', () => {
		expect(isHeicPicture(new Blob(['x'], { type: 'image/heic' }))).toBe(true);
		expect(isHeicPicture(new Blob(['x'], { type: 'IMAGE/HEIF; charset=binary' }))).toBe(true);
	});

	test('is told by its file name when the system gave it no type', () => {
		expect(isHeicPicture(new File(['x'], 'IMG_0001.HEIC', { type: '' }))).toBe(true);
		expect(isHeicPicture(new File(['x'], 'notes.txt', { type: '' }))).toBe(false);
	});

	test('is not one every browser already shows, whatever it is named', () => {
		expect(isHeicPicture(new File(['x'], 'renamed.heic', { type: 'image/jpeg' }))).toBe(false);
		expect(isHeicPicture(new Blob(['x'], { type: 'image/png' }))).toBe(false);
	});
});

describe('a picture every browser already shows', () => {
	test('is handed back as it came, without loading the converter', async () => {
		const picture = new Blob(['x'], { type: 'image/png' });
		const file = new File(['x'], 'logo.png', { type: 'image/png' });
		expect(await pictureEveryBrowserShows(picture)).toBe(picture);
		expect(await pictureFileEveryBrowserShows(file)).toBe(file);
	});
});
