import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

async function outcome() {
	return JSON.parse(await runInItsOwnProcess(new URL('./attachment-kept-file.fixture.ts', import.meta.url), 'browser'));
}

describe('a messenger picture kept on the device', () => {
	test('opens from the device without copying or signing, and a new picture is kept after it is signed', async () => {
		expect(await outcome()).toEqual({
			keptOpenable: 'blob:https://workspace.example.test/12',
			keptWork: { signings: 0, copies: 0 },
			newOpenable: 'signed-2',
			documentOpenable: 'signed-1',
			fetched: ['signed-2'],
			newKept: true,
			documentKept: false
		});
	}, ownProcessTimeoutMilliseconds);
});
