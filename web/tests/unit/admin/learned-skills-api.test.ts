import { describe, expect, test } from 'bun:test';
import { readLearnedSkillInventory } from '../../../src/routes/admin/learned-skills-api';

describe('learned skill inventory', () => {
	test('keeps valid skills and reports malformed entries', () => {
		const inventory = readLearnedSkillInventory({
			skills: [{ id: 'skill:one', version: 2, audience: 'company', description: 'Purpose', instruction: 'Do the work.', evidenceIDs: [], verification: 'evidence-reviewed', status: 'active', protected: false, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' }, { id: 'skill:two', version: 3 }],
			activeCount: 1,
			activeLimit: 20
		});
		expect(inventory.skills).toHaveLength(1);
		expect(inventory.skills[0]?.id).toBe('skill:one');
		expect(inventory.invalidSkillCount).toBe(1);
		expect(inventory.activeCount).toBe(1);
		expect(inventory.activeLimit).toBe(20);
	});

	test('rejects an invalid response instead of presenting an empty successful list', () => {
		expect(() => readLearnedSkillInventory({ skills: 'unavailable' })).toThrow('malformed');
	});

	test('rejects skills with missing required fields while preserving valid entries', () => {
		const inventory = readLearnedSkillInventory({ skills: [{ id: 'skill:one', version: 1, audience: 'company', description: 'Purpose', instruction: 'Do the work.', evidenceIDs: [], verification: 'evidence-reviewed', status: 'active', protected: false, createdAt: '2026-01-01', updatedAt: '2026-01-01' }, { id: 'skill:two', version: 2, audience: 'company' }] });
		expect(inventory.skills.map((skill) => skill.id)).toEqual(['skill:one']);
		expect(inventory.invalidSkillCount).toBe(1);
	});
});
