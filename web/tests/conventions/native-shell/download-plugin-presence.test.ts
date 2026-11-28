import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { fileDownloadPresenceMarker } from '$lib/native-shell/file-download';
import { mediaSavePresenceMarker } from '$lib/native-shell/media-save';
import { shareSheetPresenceMarker } from '$lib/native-shell/share-sheet';

const plugins = [
	{ source: '../.dependency/internkim-app/src-tauri/plugins/media-save/src/lib.rs', marker: mediaSavePresenceMarker },
	{ source: '../.dependency/internkim-app/src-tauri/plugins/share-sheet/src/lib.rs', marker: shareSheetPresenceMarker },
	{ source: '../.dependency/internkim-app/src-tauri/plugins/file-download/src/phone.rs', marker: fileDownloadPresenceMarker }
];

function announcedMarkerOf(rust: string): string {
	const match = rust.match(/PRESENCE_SCRIPT: &str = "Object\.defineProperty\(window, '(\w+)', \{ value: true \}\);"/);
	if (!match) throw new Error('the plugin announces no presence marker');
	return match[1];
}

describe('a download plugin announces itself under the name the page looks for', () => {
	for (const plugin of plugins) {
		test(plugin.source, () => {
			expect(announcedMarkerOf(readFileSync(plugin.source, 'utf8'))).toBe(plugin.marker);
		});
	}
});
