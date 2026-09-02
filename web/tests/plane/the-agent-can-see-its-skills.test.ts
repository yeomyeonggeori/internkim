import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { aCompanyPlane } from './a-company-plane';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');

type SkillInventory = { skills: { name: string; path: string }[] };

test('the agent on the plane can see every skill the plugin carries', async () => {
	const plane = await aCompanyPlane();
	try {
		const answer = await fetch(`${plane.blueclawURL}/admin/api/skills`);
		expect(answer.ok).toBe(true);
		const inventory = (await answer.json()) as SkillInventory;
		const names = inventory.skills.map((skill) => skill.name);

		for (const skillName of ['internkim-task', 'presentation', 'internkim-api']) {
			expect(
				names,
				`the plane shipped no ${skillName}: host/Dockerfile copies binaries only and the ` +
					`workspace volume is empty, so the agent answers every request without it`
			).toContain(skillName);
		}
		expect(new Set(names).size, `a skill read once per instruction root is in the prompt twice`).toBe(
			names.length
		);
	} finally {
		await plane.stop();
	}
}, 180_000);

test('the company box ships the skills it starts the agent with', () => {
	const dockerfile = readFileSync(join(repositoryRoot, 'host', 'Dockerfile'), 'utf8');
	const entrypoint = readFileSync(join(repositoryRoot, 'host', 'entrypoint.sh'), 'utf8');

	expect(
		entrypoint.includes('BLUECLAW_BUNDLED_SKILLS_PATH'),
		`host/entrypoint.sh names no bundled skills root, so blueclaw looks under the empty ` +
			`/workspace volume and starts with nothing`
	).toBe(true);
	expect(
		dockerfile.includes('.dependency/internkim-plugin/skills'),
		`host/Dockerfile copies no plugin skills, so the path the entrypoint names is empty in the image`
	).toBe(true);
});
