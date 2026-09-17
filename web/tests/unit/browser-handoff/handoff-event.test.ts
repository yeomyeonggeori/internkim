import { describe, expect, test } from 'bun:test';
import { handoffEventOf } from '../../../src/lib/browser-handoff/handoff-event';

describe('handoffEventOf', () => {
	test('reads a frame of the handed-over browser', () => {
		expect(
			handoffEventOf({ kind: 'browser.handoff.frame', handoffID: 'handoff-1', image: 'jpeg', width: 1280, height: 800, url: 'https://example.com/' })
		).toEqual({ kind: 'frame', handoffID: 'handoff-1', image: 'jpeg', width: 1280, height: 800, url: 'https://example.com/' });
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
