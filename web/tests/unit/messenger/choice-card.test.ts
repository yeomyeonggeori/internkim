import { describe, expect, test } from 'bun:test';
import {
	choiceCardOf,
	pressedChoicesStillAwaited,
	textWithoutOptionLines,
	type ChoiceCard
} from '../../../src/lib/components/channel/choice-card';
import { agentReplyTimeoutMs } from '../../../src/lib/components/channel/agent-working';
import type { ChannelInteraction } from '../../../src/lib/components/channel/channel-api';

const reader = 'member:reader';
const approvalLabels = { approve: '승인', reject: '거절' };
const pressedNow = (key: string) => ({ key, pressedAt: 0 });

function question(fields: Partial<ChannelInteraction> = {}): ChannelInteraction {
	return {
		interactionID: 'interaction-1',
		kind: 'choice',
		targetID: reader,
		options: [
			{ key: 'morning', label: '오전 10시', isRecommended: true },
			{ key: 'afternoon', label: '오후 3시', isRecommended: false }
		],
		...fields
	};
}

const approval = question({
	kind: 'approval',
	options: [
		{ key: 'approve', label: '', isRecommended: false },
		{ key: 'reject', label: '', isRecommended: false }
	]
});

const looksOf = (card: ChoiceCard) => card.options.map((option) => option.look);

const options = question().options;

describe('the option lines a question carries in its body', () => {
	test('are hidden with the blank line before them when they close the body exactly', () => {
		const body = '회의 시간을 정해야 해요.\n\n언제가 좋을까요?\n\n1. 오전 10시\n2. 오후 3시';

		expect(textWithoutOptionLines(body, options)).toBe('회의 시간을 정해야 해요.\n\n언제가 좋을까요?');
	});

	test('stay when one line differs from its option', () => {
		const body = '언제가 좋을까요?\n\n1. 오전 10시\n2. 오후 4시';

		expect(textWithoutOptionLines(body, options)).toBe(body);
	});

	test('stay when they are out of order', () => {
		const body = '언제가 좋을까요?\n\n1. 오후 3시\n2. 오전 10시';

		expect(textWithoutOptionLines(body, options)).toBe(body);
	});

	test('stay when something follows them', () => {
		const body = '언제가 좋을까요?\n\n1. 오전 10시\n2. 오후 3시\n\n답 주세요.';

		expect(textWithoutOptionLines(body, options)).toBe(body);
	});

	test('leave nothing when the body was only the options', () => {
		expect(textWithoutOptionLines('1. 오전 10시\n2. 오후 3시', options)).toBe('');
	});

	test('are only the labelled options, so an approval without labels hides nothing', () => {
		const body = '견적서를 보낼까요?\n\n1. 승인\n2. 거절';

		expect(textWithoutOptionLines(body, approval.options)).toBe(body);
	});
});

describe('a choice card', () => {
	test('lets the person asked press any option, and marks the recommended one', () => {
		const card = choiceCardOf(question(), reader, undefined, approvalLabels);

		expect(card.isOpen).toBe(true);
		expect(card.options.map(({ key, position, look, isRecommended }) => ({ key, position, look, isRecommended }))).toEqual([
			{ key: 'morning', position: 1, look: 'pressable', isRecommended: true },
			{ key: 'afternoon', position: 2, look: 'pressable', isRecommended: false }
		]);
	});

	test('is read-only to anyone else in the conversation', () => {
		const card = choiceCardOf(question({ targetID: 'member:someone-else' }), reader, undefined, approvalLabels);

		expect(card.isOpen).toBe(true);
		expect(looksOf(card)).toEqual(['readOnly', 'readOnly']);
	});

	test('is read-only to a reader the app does not know yet', () => {
		expect(looksOf(choiceCardOf(question(), '', undefined, approvalLabels))).toEqual(['readOnly', 'readOnly']);
	});

	test('is read-only to everyone when it names nobody, since only the person asked is heard', () => {
		const card = choiceCardOf(question({ targetID: '' }), reader, undefined, approvalLabels);

		expect(looksOf(card)).toEqual(['readOnly', 'readOnly']);
	});

	test('shows the pressed option as chosen until the answer is recorded', () => {
		const card = choiceCardOf(question(), reader, pressedNow('afternoon'), approvalLabels);

		expect(card.isOpen).toBe(false);
		expect(looksOf(card)).toEqual(['faded', 'chosen']);
	});

	test('checks the option the record names, whoever reads it', () => {
		const resolved = question({ resolution: { key: 'morning' } });

		expect(looksOf(choiceCardOf(resolved, reader, undefined, approvalLabels))).toEqual(['chosen', 'faded']);
		expect(looksOf(choiceCardOf(resolved, 'member:someone-else', undefined, approvalLabels))).toEqual(['chosen', 'faded']);
	});

	test('takes the record over what this screen pressed', () => {
		const card = choiceCardOf(question({ resolution: { key: 'morning' } }), reader, pressedNow('afternoon'), approvalLabels);

		expect(looksOf(card)).toEqual(['chosen', 'faded']);
	});

	test('checks nothing when the answer was in the person’s own words', () => {
		const card = choiceCardOf(question({ resolution: {} }), reader, undefined, approvalLabels);

		expect(card.isOpen).toBe(false);
		expect(looksOf(card)).toEqual(['faded', 'faded']);
	});

	test('names an approval’s options in the reader’s language and lays them side by side, the refusal declining', () => {
		const card = choiceCardOf(approval, reader, undefined, approvalLabels);

		expect(card.layout).toBe('pair');
		expect(card.options.map(({ key, label, isDeclining }) => ({ key, label, isDeclining }))).toEqual([
			{ key: 'approve', label: '승인', isDeclining: false },
			{ key: 'reject', label: '거절', isDeclining: true }
		]);
		expect(choiceCardOf(question(), reader, undefined, approvalLabels).layout).toBe('list');
	});

	test('a settled approval stays a pair', () => {
		const card = choiceCardOf({ ...approval, resolution: { key: 'reject' } }, reader, undefined, approvalLabels);

		expect(card.layout).toBe('pair');
		expect(looksOf(card)).toEqual(['faded', 'chosen']);
	});

	test('does not draw an option with neither a label nor a key it can name', () => {
		const card = choiceCardOf(
			question({ options: [{ key: 'unknown', label: '', isRecommended: false }, ...options] }),
			reader,
			undefined,
			approvalLabels
		);

		expect(card.options.map(({ key, position }) => ({ key, position }))).toEqual([
			{ key: 'morning', position: 1 },
			{ key: 'afternoon', position: 2 }
		]);
	});
});

describe('an option pressed on this screen', () => {
	test('is held while the agent may still be answering and let go when it is no longer expected to', () => {
		const pressed = new Map([['interaction-1', { key: 'morning', pressedAt: 1_000 }]]);

		expect([...pressedChoicesStillAwaited(pressed, 1_000 + agentReplyTimeoutMs - 1).keys()]).toEqual(['interaction-1']);
		expect([...pressedChoicesStillAwaited(pressed, 1_000 + agentReplyTimeoutMs).keys()]).toEqual([]);
	});

	test('stays the very same record while nothing has expired, so a screen reading it is not woken again', () => {
		const pressed = new Map([['interaction-1', { key: 'morning', pressedAt: 1_000 }]]);

		expect(pressedChoicesStillAwaited(pressed, 1_001)).toBe(pressed);
	});
});
