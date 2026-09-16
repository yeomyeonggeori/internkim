import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/lib/organization/types';
import {
	canEditDataRoomClearance,
	dataRoomClearanceLabel,
	offeredDataRoomClearances
} from '../../../src/routes/organization/organization-clearance-model';
import { organizationDirectoryText } from '../../../src/routes/organization/text';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		memberID: '',
		handle: '',
		name: '',
		email: '',
		...overrides
	};
}

const representative = userRecord({ memberID: 'ceo', role: 'admin', clearance: 3 });
const managingAdministrator = userRecord({ memberID: 'cfo', role: 'admin', clearance: 2 });
const managingMember = userRecord({ memberID: 'lead', role: 'member', clearance: 2 });
const member = userRecord({ memberID: 'intern', role: 'member', clearance: 1 });

describe('who may set a data room clearance', () => {
	test('an administrator sets anyone below their own', () => {
		expect(canEditDataRoomClearance(representative, managingAdministrator)).toBe(true);
		expect(canEditDataRoomClearance(representative, member)).toBe(true);
		expect(canEditDataRoomClearance(managingAdministrator, member)).toBe(true);
	});

	test('an administrator sets their own', () => {
		expect(canEditDataRoomClearance(representative, representative)).toBe(true);
		expect(canEditDataRoomClearance(managingAdministrator, managingAdministrator)).toBe(true);
	});

	test('nobody raises a peer or a superior', () => {
		expect(canEditDataRoomClearance(managingAdministrator, managingMember)).toBe(false);
		expect(canEditDataRoomClearance(managingAdministrator, representative)).toBe(false);
	});

	test('a non-administrator sets nobody, themselves included', () => {
		expect(canEditDataRoomClearance(managingMember, member)).toBe(false);
		expect(canEditDataRoomClearance(managingMember, managingMember)).toBe(false);
	});

	test('an unknown viewer or target sets nothing', () => {
		expect(canEditDataRoomClearance(undefined, member)).toBe(false);
		expect(canEditDataRoomClearance(representative, undefined)).toBe(false);
	});

	test('a record without a clearance counts as the lowest', () => {
		const unanswered = userRecord({ memberID: 'new', role: 'member' });
		expect(canEditDataRoomClearance(managingAdministrator, unanswered)).toBe(true);
		expect(canEditDataRoomClearance(userRecord({ memberID: 'admin', role: 'admin' }), member)).toBe(false);
	});
});

describe('the clearances an administrator may choose from', () => {
	test('are capped at their own', () => {
		expect(offeredDataRoomClearances(representative)).toEqual([1, 2, 3]);
		expect(offeredDataRoomClearances(managingAdministrator)).toEqual([1, 2]);
		expect(offeredDataRoomClearances(userRecord({ memberID: 'admin', role: 'admin', clearance: 1 }))).toEqual([1]);
	});

	test('fall back to the lowest for an unknown viewer', () => {
		expect(offeredDataRoomClearances(undefined)).toEqual([1]);
	});
});

describe('the clearance label', () => {
	test('shows the number with its word in each locale', () => {
		expect(dataRoomClearanceLabel(1, organizationDirectoryText.ko)).toBe('1 구성원');
		expect(dataRoomClearanceLabel(2, organizationDirectoryText.ko)).toBe('2 경영진');
		expect(dataRoomClearanceLabel(3, organizationDirectoryText.ko)).toBe('3 대표·이사회');
		expect(dataRoomClearanceLabel(3, organizationDirectoryText.en)).toBe('3 representative');
	});

	test('shows a value it has no word for as the bare number', () => {
		expect(dataRoomClearanceLabel(7, organizationDirectoryText.en)).toBe('7');
	});
});
