import { describe, expect, test } from 'bun:test';
import { personKey, personLabel, type MessengerDirectory } from '../../../src/lib/messenger/messenger-directory';

function directoryOf(): MessengerDirectory {
	return {
		nameOfMember: new Map([['m1', '이샘플']]),
		nameOfExternal: new Map([
			['U123', '김철수'],
			['U999', '김철수'],
			['UBOT', '다른 에이전트']
		]),
		externalOfMember: new Map(),
		memberOfExternal: new Map([['U777', 'm1']]),
		memberOfEmail: new Map()
	};
}

describe('personKey', () => {
	test('a member is identified by their member', () => {
		expect(personKey({ memberID: 'm1' })).toBe('member:m1');
	});

	test('anyone else is identified by the platform id, not their name', () => {
		expect(personKey({ externalID: 'U123' })).toBe('external:U123');
	});

	test('two people sharing a name stay apart', () => {
		expect(personKey({ externalID: 'U123' })).not.toBe(personKey({ externalID: 'U999' }));
	});
});

describe('personLabel', () => {
	test('a member is named by the directory', () => {
		expect(personLabel({ memberID: 'm1' }, directoryOf())).toBe('이샘플');
	});

	test('a platform id that belongs to a member is named as that member', () => {
		expect(personLabel({ externalID: 'U777' }, directoryOf())).toBe('이샘플');
	});

	test('someone who is nobody here keeps the name the platform gives them', () => {
		expect(personLabel({ externalID: 'UBOT' }, directoryOf())).toBe('다른 에이전트');
	});

	test('an unknown person is blank rather than a raw id', () => {
		expect(personLabel({ externalID: 'UNSEEN' }, directoryOf())).toBe('');
	});
});
