import { describe, expect, test } from 'bun:test';
import { externalIDFor, type MessengerDirectory } from '../../../src/lib/messenger/messenger-directory';

function directoryOf(): MessengerDirectory {
	return {
		nameOfMember: new Map([['m1', '이샘플']]),
		nameOfExternal: new Map([['U1', '이샘플']]),
		memberOfExternal: new Map([['U1', 'm1']]),
		externalOfMember: new Map([['m1', 'U1']]),
		memberOfEmail: new Map([['sample@dawn.kim', 'm1']])
	};
}

describe('externalIDFor', () => {
	test('a member id reaches the picture the messenger holds', () => {
		expect(externalIDFor({ memberID: 'm1' }, directoryOf())).toBe('U1');
	});

	test('an email reaches the same picture, whatever its casing', () => {
		expect(externalIDFor({ email: '  Sample@Dawn.kim ' }, directoryOf())).toBe('U1');
	});

	test('someone the messenger does not know has no picture', () => {
		expect(externalIDFor({ email: 'stranger@dawn.kim' }, directoryOf())).toBe('');
		expect(externalIDFor({ memberID: 'm2' }, directoryOf())).toBe('');
		expect(externalIDFor({}, directoryOf())).toBe('');
	});
});
