import { describe, expect, test } from 'bun:test';
import { circlesByMemberID } from '../../../src/lib/server/member-directory';

describe('the circles a member holds', () => {
	test('are the data room roles granted to them', () => {
		const held = circlesByMemberID([
			{ member_id: 'sample', role_code: 'leadership' },
			{ member_id: 'sample', role_code: 'member' },
			{ member_id: 'example', role_code: 'member' }
		]);

		expect(held.get('sample')).toEqual(['leadership', 'member']);
		expect(held.get('example')).toEqual(['member']);
	});

	test('name a role once when it is granted twice', () => {
		const held = circlesByMemberID([
			{ member_id: 'sample', role_code: 'hr' },
			{ member_id: 'sample', role_code: 'hr' }
		]);

		expect(held.get('sample')).toEqual(['hr']);
	});

	test('leave out a grant that names no member', () => {
		expect(circlesByMemberID([{ member_id: null, role_code: 'investor' }]).size).toBe(0);
	});
});
