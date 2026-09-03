import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { readSkillInventory, skillRootOf, skillRootsOf } from '../../../src/routes/admin/skills-api';

const relayAnswer = {
	skills: [
		{
			name: 'internkim-api',
			description: 'Read and change a company workspace over its public API.',
			path: '/delivery/skills/internkim-api',
			toolReferences: ['shell', 'web_fetch']
		},
		{ name: 'calendar', description: '', path: '/delivery/skills/calendar' },
		{ name: 'agent-browser', description: 'Drive a browser.', path: '/workspace/skills/agent-browser', toolReferences: [] },
		{ name: 42, path: '/delivery/skills/nonsense' },
		'not a skill at all'
	],
	unavailableSkills: [
		{
			name: 'weather',
			description: 'Look up a forecast.',
			path: '/delivery/skills/weather',
			missingEnvironmentVariables: ['OPEN_METEO_BASE_URL'],
			missingToolNames: []
		}
	]
};

describe('the skill inventory, read out of a relay answer', () => {
	test('keeps every named skill and drops what is not one', () => {
		const inventory = readSkillInventory(relayAnswer);

		expect(inventory.skills.map((skill) => skill.name)).toEqual(['internkim-api', 'calendar', 'agent-browser']);
		expect(inventory.unavailableSkills.map((skill) => skill.name)).toEqual(['weather']);
	});

	test('a skill that declares no tools reads as declaring none, never as undefined', () => {
		const [firstSkill, secondSkill] = readSkillInventory(relayAnswer).skills;

		expect(firstSkill.toolReferences).toEqual(['shell', 'web_fetch']);
		expect(secondSkill.toolReferences).toEqual([]);
		expect(secondSkill.description).toBe('');
	});

	test('an unavailable skill keeps what this computer is missing', () => {
		const [unavailable] = readSkillInventory(relayAnswer).unavailableSkills;

		expect(unavailable.missingEnvironmentVariables).toEqual(['OPEN_METEO_BASE_URL']);
		expect(unavailable.missingToolNames).toEqual([]);
	});

	test('an answer with no lists at all reads as an empty inventory', () => {
		expect(readSkillInventory(null)).toEqual({ skills: [], unavailableSkills: [] });
		expect(readSkillInventory({})).toEqual({ skills: [], unavailableSkills: [] });
		expect(readSkillInventory({ skills: 'none' })).toEqual({ skills: [], unavailableSkills: [] });
	});
});

describe('the root each skill was read from', () => {
	test('is the directory holding it', () => {
		expect(skillRootOf('/delivery/skills/internkim-api')).toBe('/delivery/skills');
		expect(skillRootOf('/delivery/skills/internkim-api/')).toBe('/delivery/skills');
		expect(skillRootOf('internkim-api')).toBe('internkim-api');
	});

	test('groups the list so a root the agent never opened is visible as an absent one', () => {
		const roots = skillRootsOf(readSkillInventory(relayAnswer).skills);

		expect(roots.map((root) => root.path)).toEqual(['/delivery/skills', '/workspace/skills']);
		expect(roots[0].skills.map((skill) => skill.name)).toEqual(['internkim-api', 'calendar']);
		expect(roots[1].skills.map((skill) => skill.name)).toEqual(['agent-browser']);
	});

	test('keeps the order the agent reported, so the first root stays first', () => {
		const roots = skillRootsOf([
			{ name: 'b', description: '', path: '/second/b', toolReferences: [] },
			{ name: 'a', description: '', path: '/first/a', toolReferences: [] },
			{ name: 'c', description: '', path: '/second/c', toolReferences: [] }
		]);

		expect(roots.map((root) => root.path)).toEqual(['/second', '/first']);
		expect(roots[0].skills.map((skill) => skill.name)).toEqual(['b', 'c']);
	});
});

describe('the way the section reaches the inventory', () => {
	test('names a capability the relay carries, at the path admind answers on', () => {
		const section = readFileSync('src/routes/admin/skills-api.ts', 'utf8');
		const relay = readFileSync('../host/relay/forward.ts', 'utf8');

		expect(section).toContain("capability: 'person.skills.list'");
		expect(section).toContain("adminApiFetch('/skills/api')");
		expect(relay).toContain("'person.skills.list': '/skills/api'");
	});
});
