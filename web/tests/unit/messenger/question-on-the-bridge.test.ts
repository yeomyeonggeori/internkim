import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

const options = [
	{ key: 'morning', label: '오전 10시', isRecommended: true },
	{ key: 'afternoon', label: '오후 3시', isRecommended: false }
];

async function outcome() {
	return JSON.parse(await runInItsOwnProcess(new URL('./question-on-the-bridge.fixture.ts', import.meta.url), 'browser'));
}

describe('a question and a failure read from person.messages.list', () => {
	test('reach the screen as an interaction naming its target the way the reader is named, and as an error', async () => {
		const read = await outcome();

		expect(read.currentUserID).toBe('member:reader');
		expect(read.messages).toEqual({
			'asks-the-reader': {
				interaction: { interactionID: 'interaction-1', kind: 'choice', targetID: 'member:reader', options },
				isError: false
			},
			'asks-a-colleague': {
				interaction: {
					interactionID: 'interaction-2',
					kind: 'approval',
					targetID: 'member:colleague',
					options: [
						{ key: 'approve', label: '', isRecommended: false },
						{ key: 'reject', label: '', isRecommended: false }
					]
				},
				isError: false
			},
			'asks-anyone': {
				interaction: {
					interactionID: 'interaction-3',
					kind: 'choice',
					targetID: '',
					options,
					resolution: { key: 'morning' }
				},
				isError: false
			},
			'answered-in-own-words': {
				interaction: { interactionID: 'interaction-4', kind: 'choice', targetID: 'member:reader', options, resolution: {} },
				isError: false
			},
			'asks-in-a-broken-shape': { interaction: null, isError: false },
			failed: { interaction: null, isError: true },
			plain: { interaction: null, isError: false }
		});
		expect(read.warnings).toEqual(['a message asked a question in a shape this app cannot draw']);
	}, ownProcessTimeoutMilliseconds);

	test('a pressed option is sent as its label with the answer it gives', async () => {
		const read = await outcome();

		expect(read.sentBody).toBe('오전 10시');
		expect(read.sentAnswer).toEqual({ interactionID: 'interaction-1', key: 'morning' });
	}, ownProcessTimeoutMilliseconds);
});
