import { describe, expect, test } from 'bun:test';
import { circlesByMemberID } from '../../../src/lib/server/member-directory';

describe('the circles a member holds', () => {
	test('are the circles they belong to, grouped by member', () => {
		const held = circlesByMemberID([
			{ member_id: 'sample', circle_id: 'leadership' },
			{ member_id: 'sample', circle_id: 'member' },
			{ member_id: 'example', circle_id: 'member' }
		]);

		expect(held.get('sample')).toEqual(['leadership', 'member']);
		expect(held.get('example')).toEqual(['member']);
	});
});
