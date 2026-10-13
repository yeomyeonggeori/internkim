import { describe, expect, test } from 'bun:test';
import { filesToAttachFromClipboard } from '../../../src/lib/components/channel/clipboard-files';

function clipboardOf(files: File[], words: string) {
	return { files: Object.assign(files, { item: (index: number) => files[index] ?? null }), getData: () => words };
}

const screenshot = new File(['png'], 'screenshot.png', { type: 'image/png' });

describe('what a paste attaches', () => {
	test('a picture on its own is attached', () => {
		expect(filesToAttachFromClipboard(clipboardOf([screenshot], ''))).toEqual([screenshot]);
	});

	test('words beside a picture are pasted as words, so nothing is attached', () => {
		expect(filesToAttachFromClipboard(clipboardOf([screenshot], '표 한 칸'))).toEqual([]);
	});

	test('words alone attach nothing', () => {
		expect(filesToAttachFromClipboard(clipboardOf([], '글자'))).toEqual([]);
	});

	test('an empty clipboard attaches nothing', () => {
		expect(filesToAttachFromClipboard(null)).toEqual([]);
	});
});
