import { describe, expect, test } from 'bun:test';
import { replyFromGuestResult } from '../../pilot/arms/bluecollar';

describe('the Bluecollar pilot reply', () => {
	test('keeps the result for a completed guest task', () => {
		const reply = replyFromGuestResult({
			status: 'completed',
			result: '두 조직의 중요도를 낮췄습니다.',
			failureReason: 'Internal completion detail',
		});

		expect(reply).toBe('두 조직의 중요도를 낮췄습니다.');
	});

	test('uses the pending question instead of its internal failure reason', () => {
		const reply = replyFromGuestResult({
			status: 'waiting_user_input',
			result: '두 거래의 새 담당자로 누구를 지정할까요?',
			failureReason: 'Requester asked to replace managers but did not specify who.',
		});

		expect(reply).toBe('두 거래의 새 담당자로 누구를 지정할까요?');
	});

	test('falls back to the stored question when the waiting task result is empty', () => {
		const reply = replyFromGuestResult({
			status: 'waiting_user_input',
			result: '',
			failureReason: 'Who should own the records?',
		});

		expect(reply).toBe('Who should own the records?');
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
