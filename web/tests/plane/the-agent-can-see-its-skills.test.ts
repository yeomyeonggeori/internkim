import { expect, test } from 'bun:test';
import { readdirSync } from 'node:fs';
import { aCompanyPlane } from './a-company-plane';

type SkillInventory = {
	skills: { name: string; path: string }[];
	unavailableSkills: {
		name: string;
		path: string;
		missingEnvironmentVariables: string[];
		missingToolNames?: string[];
	}[];
};

function theSkillsTheBoxCarries(): string[] {
	const shippedSkillsPath = process.env.COMPANY_PLANE_SKILLS;
	if (!shippedSkillsPath) throw new Error('COMPANY_PLANE_SKILLS is not set');
	return readdirSync(shippedSkillsPath, { withFileTypes: true })
		.filter((entry) => entry.isDirectory())
		.map((entry) => entry.name);
}

test('the agent on the plane can see every skill it can run, and no other', async () => {
	const plane = await aCompanyPlane();
	try {
		const answer = await fetch(`${plane.blueclawURL}/admin/api/skills`);
		expect(answer.ok).toBe(true);
		const inventory = (await answer.json()) as SkillInventory;
		const names = inventory.skills.map((skill) => skill.name);

		for (const skillName of ['internkim-task', 'presentation']) {
			expect(
				names,
				`the plane shipped no ${skillName}: host/Dockerfile copies binaries only and the ` +
					`workspace volume is empty, so the agent answers every request without it`
			).toContain(skillName);
		}
		expect(new Set(names).size, `a skill read once per instruction root is in the prompt twice`).toBe(
			names.length
		);

		expect(
			inventory.unavailableSkills.map((skill) => skill.name),
			`a plugin skill the plane cannot run is held out of the prompt, and every one it ships runs here`
		).toEqual([]);
		expect(
			[...names, ...inventory.unavailableSkills.map((skill) => skill.name)].sort(),
			`the host ships every plugin skill to the box and only the prompt is smaller, so a ` +
				`skill in neither list never reached disk`
		).toEqual(theSkillsTheBoxCarries().sort());
	} finally {
		await plane.stop();
	}
}, 180_000);
