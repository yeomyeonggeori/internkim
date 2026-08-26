import { describe, expect, test } from 'bun:test';
import { circleNamesByMemberID } from '../../../src/lib/server/fleet-user-directory';

describe('the circles a company keeps', () => {
	test('gives each member the circles that name them', () => {
		const held = circleNamesByMemberID([
			{ name: 'admin', circle_member: [{ member_id: 'lee' }, { member_id: 'rain' }] },
			{ name: 'c-level', circle_member: [{ member_id: 'lee' }] },
			{ name: 'staff', circle_member: [{ member_id: 'lee' }, { member_id: 'rain' }, { member_id: 'kwak' }] }
		]);

		expect(held.get('lee')).toEqual(['admin', 'c-level', 'staff']);
		expect(held.get('rain')).toEqual(['admin', 'staff']);
		expect(held.get('kwak')).toEqual(['staff']);
	});

	test('leaves out a member no circle names, rather than giving them an empty one', () => {
		const held = circleNamesByMemberID([{ name: 'c-level', circle_member: [{ member_id: 'lee' }] }]);

		expect(held.has('kwak')).toBe(false);
	});

	test('reads a circle nobody is in', () => {
		const held = circleNamesByMemberID([
			{ name: 'representative', circle_member: [] },
			{ name: 'hr-compensation', circle_member: null }
		]);

		expect(held.size).toBe(0);
	});
});
