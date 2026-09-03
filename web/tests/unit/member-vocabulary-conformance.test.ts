import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';
import { memberRoles, memberStatuses } from '../../src/lib/member-vocabulary';
import {
	memberRoles as sharedMemberRoles,
	memberStatuses as sharedMemberStatuses
} from '../../../supabase/functions/_shared/member-vocabulary.ts';
import { allUserRoleOptions } from '../../src/routes/admin/users-section-policy';
import { adminText } from '../../src/routes/admin/text';

const pinnedPath = join(import.meta.dir, '../../../supabase/tests/member_role_and_status_are_declared_once.test.sql');

function statusesTheDatabaseIsPinnedTo(): string[] {
	const pinned = readFileSync(pinnedPath, 'utf8');
	const declaration = pinned.match(/array\[([^\]]+)\]/);
	if (!declaration) throw new Error('the pgTAP test no longer pins the member status enum to an array literal');
	return declaration[1]
		.split(',')
		.map((value) => value.trim().replace(/^'|'$/g, ''))
		.sort();
}

function theDatabasePinsTheRoleToABoolean(): boolean {
	return readFileSync(pinnedPath, 'utf8').includes("'member', 'is_admin', 'boolean'");
}

describe('the shared member vocabulary stays interchangeable with the web one', () => {
	test('both name the same roles', () => {
		expect([...memberRoles]).toEqual([...sharedMemberRoles]);
	});

	test('both name the same statuses', () => {
		expect([...memberStatuses]).toEqual([...sharedMemberStatuses]);
	});
});

describe('the record is what the vocabulary is read from', () => {
	test('the member status enum names the same statuses', () => {
		expect(statusesTheDatabaseIsPinnedTo()).toEqual([...memberStatuses].sort());
	});

	test('a boolean column can only mean two roles', () => {
		expect(theDatabasePinsTheRoleToABoolean()).toBe(true);
		expect(memberRoles).toHaveLength(2);
	});
});

describe('the admin console offers the roles the record can hold', () => {
	test('every role option is a declared role, and every declared role is offered', () => {
		expect(allUserRoleOptions(adminText.ko).map((option) => option.value).sort()).toEqual(
			[...memberRoles].sort()
		);
	});
});
