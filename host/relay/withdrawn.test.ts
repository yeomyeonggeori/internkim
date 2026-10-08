import { describe, expect, test } from 'bun:test';
import { readWithdrawal, withdrawalTeller, withdrawRequestOf, WithdrawnMessages, type WithdrawRequest } from './withdrawn';

const takenBack = {
	conversationID: 'channel-1',
	messageID: 'post-7',
	authorExternalID: 'U-author',
	recipientExternalIDs: ['U-author', 'U-first', 'U-second']
};

describe('readWithdrawal', () => {
	test('those who were told about the message are told it was taken back, not its author', () => {
		expect(readWithdrawal(takenBack)).toEqual({
			conversationID: 'channel-1',
			messageID: 'post-7',
			recipientExternalIDs: ['U-first', 'U-second']
		});
	});

	test('a withdrawal that does not name the message is not one', () => {
		expect(readWithdrawal({ ...takenBack, messageID: '' })).toBeNull();
		expect(readWithdrawal('post-7')).toBeNull();
	});
});

describe('withdrawRequestOf', () => {
	test('the project is told which message to take off whose devices', () => {
		expect(withdrawRequestOf(readWithdrawal(takenBack)!, 'buzz')).toEqual({
			platform: 'buzz',
			externalIDs: ['U-first', 'U-second'],
			conversationID: 'channel-1',
			messageID: 'post-7'
		});
	});
});

describe('WithdrawnMessages', () => {
	test('remembers what was taken back in each conversation, once each', () => {
		const withdrawn = new WithdrawnMessages();
		withdrawn.remember(readWithdrawal(takenBack)!);
		withdrawn.remember(readWithdrawal({ ...takenBack, messageID: 'post-8' })!);
		withdrawn.remember(readWithdrawal(takenBack)!);
		withdrawn.remember(readWithdrawal({ ...takenBack, conversationID: 'channel-2' })!);

		expect(withdrawn.in('channel-1')).toEqual(['post-8', 'post-7']);
		expect(withdrawn.in('channel-3')).toEqual([]);
	});

	test('keeps only the most recent of a conversation', () => {
		const withdrawn = new WithdrawnMessages();
		for (let index = 0; index < 25; index += 1) {
			withdrawn.remember(readWithdrawal({ ...takenBack, messageID: `post-${index}` })!);
		}

		expect(withdrawn.in('channel-1')).toHaveLength(20);
		expect(withdrawn.in('channel-1')[0]).toBe('post-5');
	});
});

describe('withdrawalTeller', () => {
	test('asks the project to take the notification off the phones of those told, and remembers it', async () => {
		const withdrawn = new WithdrawnMessages();
		const asked: WithdrawRequest[] = [];
		const tell = withdrawalTeller({
			withdrawn,
			platform: 'buzz',
			askTheProject: async (request) => {
				asked.push(request);
				return { reached: 2 };
			}
		});

		expect(await tell(readWithdrawal(takenBack)!)).toBe(2);
		expect(asked).toEqual([withdrawRequestOf(readWithdrawal(takenBack)!, 'buzz')]);
		expect(withdrawn.in('channel-1')).toEqual(['post-7']);
	});

	test('remembers the message even when the project cannot be reached, so the next notification takes it back', async () => {
		const withdrawn = new WithdrawnMessages();
		const tell = withdrawalTeller({
			withdrawn,
			platform: 'buzz',
			askTheProject: async () => {
				throw new Error('the project answered 503 for withdraw-notification');
			}
		});

		await expect(tell(readWithdrawal(takenBack)!)).rejects.toThrow('503');
		expect(withdrawn.in('channel-1')).toEqual(['post-7']);
	});

	test('asks nothing when nobody else was told', async () => {
		const asked: WithdrawRequest[] = [];
		const tell = withdrawalTeller({
			withdrawn: new WithdrawnMessages(),
			platform: 'buzz',
			askTheProject: async (request) => {
				asked.push(request);
				return {};
			}
		});

		expect(await tell(readWithdrawal({ ...takenBack, recipientExternalIDs: ['U-author'] })!)).toBe(0);
		expect(asked).toEqual([]);
	});
});
