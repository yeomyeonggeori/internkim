import { describe, expect, test } from 'bun:test';
import { handoffEventOf } from '../../../src/lib/browser-handoff/handoff-event';

describe('handoffEventOf', () => {
	test('reads a frame of the handed-over browser', () => {
		expect(
			handoffEventOf({ kind: 'browser.handoff.frame', handoffID: 'handoff-1', image: 'jpeg', width: 1280, height: 800, url: 'https://example.com/' })
		).toEqual({ kind: 'frame', handoffID: 'handoff-1', image: 'jpeg', width: 1280, height: 800, url: 'https://example.com/', fields: [] });
	});

	test('reads the places on the frame the requester can type into', () => {
		const frame = handoffEventOf({
			kind: 'browser.handoff.frame',
			handoffID: 'handoff-1',
			image: 'jpeg',
			width: 1280,
			height: 800,
			fields: [[20, 40, 200, 30], [20, 90, 0, 30], ['20', 90, 200, 30], [20, 90]]
		});

		expect(frame).toMatchObject({ fields: [{ x: 20, y: 40, width: 200, height: 30 }] });
	});

	test('reads why the browser did not take an input', () => {
		expect(handoffEventOf({ kind: 'browser.handoff.trouble', handoffID: 'handoff-1', reason: 'the device browser did not answer in time' })).toEqual({
			kind: 'trouble',
			handoffID: 'handoff-1',
			reason: 'the device browser did not answer in time'
		});
	});

	test('reads how a handoff ended', () => {
		expect(handoffEventOf({ kind: 'browser.handoff.ended', handoffID: 'handoff-1', outcome: 'expired' })).toEqual({
			kind: 'ended',
			handoffID: 'handoff-1',
			outcome: 'expired'
		});
	});

	test('leaves every other company event alone', () => {
		expect(handoffEventOf({ kind: 'message.created', conversationID: 'c' })).toBeNull();
		expect(handoffEventOf({ kind: 'browser.handoff.frame', handoffID: 'handoff-1', width: 1280, height: 800 })).toBeNull();
		expect(handoffEventOf({ kind: 'browser.handoff.ended', handoffID: 'handoff-1', outcome: 'exploded' })).toBeNull();
		expect(handoffEventOf(null)).toBeNull();
	});
});
