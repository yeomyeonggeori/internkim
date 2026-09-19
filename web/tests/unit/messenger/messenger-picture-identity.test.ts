import { describe, expect, test } from 'bun:test';
import { externalIDFor, type MessengerDirectory } from '../../../src/lib/messenger/messenger-directory';

function directoryOf(): MessengerDirectory {
	return {
		nameOfMember: new Map([['m1', '이샘플']]),
		nameOfExternal: new Map([['U1', '이샘플']]),
		memberOfExternal: new Map([['U1', 'm1']]),
		externalsOfMember: new Map([['m1', ['U1']]]),
		memberOfEmail: new Map([['sample@example.com', 'm1']]),
		adminMemberIDs: new Set<string>()
	};
}

describe('externalIDFor', () => {
	test('a member id reaches the picture the messenger holds', () => {
		expect(externalIDFor({ memberID: 'm1' }, directoryOf())).toBe('U1');
	});

	test('an email reaches the same picture, whatever its casing', () => {
		expect(externalIDFor({ email: '  Sample@Example.com ' }, directoryOf())).toBe('U1');
	});

	test('someone the messenger does not know has no picture', () => {
		expect(externalIDFor({ email: 'stranger@example.com' }, directoryOf())).toBe('');
		expect(externalIDFor({ memberID: 'm2' }, directoryOf())).toBe('');
		expect(externalIDFor({}, directoryOf())).toBe('');
	});
});
