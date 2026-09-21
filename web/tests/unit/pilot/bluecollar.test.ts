import { describe, expect, test } from 'bun:test';
import { replyFromGuestResult } from '../../pilot/arms/bluecollar';

describe('the Bluecollar pilot reply', () => {
	test('uses the pending question instead of its internal failure reason', () => {
		const reply = replyFromGuestResult({
			status: 'waiting_user_input',
			result: '두 거래의 새 담당자로 누구를 지정할까요?',
			failureReason: 'Requester asked to replace managers but did not specify who.',
		});

		expect(reply).toBe('두 거래의 새 담당자로 누구를 지정할까요?');
	});

	test('uses the failure reason for a failed guest task', () => {
		const reply = replyFromGuestResult({
			status: 'failed',
			result: 'Internal failure details',
			failureReason: 'The requested capability failed.',
		});

		expect(reply).toBe('The requested capability failed.');
	});

	test('uses the failure reason for a blocked guest task', () => {
		const reply = replyFromGuestResult({
			status: 'blocked',
			result: '',
			failureReason: 'max_elapsed',
		});

		expect(reply).toBe('max_elapsed');
	});
});
