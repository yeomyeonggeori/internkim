import { expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('a custom emoji is still drawing until its picture arrives or cannot, so nothing shows its name in between', async () => {
	const output = await runInItsOwnProcess(new URL('./custom-emoji-drawing-state.fixture.ts', import.meta.url), 'script');
	expect(JSON.parse(output)).toEqual({
		beforeTheListArrives: true, listedButNotYetDrawn: true, notInTheList: false, drawn: false, couldNotBeDrawn: false
	});
}, ownProcessTimeoutMilliseconds);
