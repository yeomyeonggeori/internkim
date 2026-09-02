import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { aCompanyPlane } from './a-company-plane';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');

type SkillInventory = { skills: { name: string; path: string }[] };

test('the agent on the plane can see a plugin skill and a workspace skill', async () => {
	const plane = await aCompanyPlane();
	try {
		const answer = await fetch(`${plane.blueclawURL}/admin/api/skills`);
		expect(answer.ok).toBe(true);
		const inventory = (await answer.json()) as SkillInventory;
		const names = inventory.skills.map((skill) => skill.name);

		expect(
			names,
			`the plane shipped no skills: host/Dockerfile copies binaries only and the workspace ` +
				`volume is empty, so the agent answers every request with none of them`
		).toContain('internkim-task');
		expect(
			names,
			`the vendored plugin's skills reached no plane, so a company plane can do less than a device`
		).toContain('presentation');
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
	for (const root of ['assets/blueclaw-workspace/skills', '.dependency/internkim-plugin/skills']) {
		expect(
			dockerfile.includes(root),
			`host/Dockerfile copies no ${root}, so the path the entrypoint names is empty in the image`
		).toBe(true);
	}
});
