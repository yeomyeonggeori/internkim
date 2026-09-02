import { expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { aCompanyPlane } from './a-company-plane';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');

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
			names,
			`internkim-api is in the prompt on a plane with no INTERNKIM_TOKEN, so the agent ` +
				`will select it and every call will end at "INTERNKIM_TOKEN is not set"`
		).not.toContain('internkim-api');
		expect(
			names,
			`create-gws-file needs the Google tools, and DefaultToolDescriptors - the set ` +
				`stamped into runtime.json - does not carry them, so the agent would select a ` +
				`skill whose every call names a tool it was never given`
		).not.toContain('create-gws-file');
		expect(inventory.unavailableSkills.map((skill) => skill.name).sort()).toEqual([
			'create-gws-file',
			'internkim-api'
		]);
		const unavailableByName = new Map(inventory.unavailableSkills.map((skill) => [skill.name, skill]));
		expect(unavailableByName.get('internkim-api')?.missingEnvironmentVariables).toEqual([
			'INTERNKIM_TOKEN'
		]);
		expect(unavailableByName.get('create-gws-file')?.missingToolNames).toEqual([
			'google_docs_create',
			'google_sheets_create',
			'google_gmail_send'
		]);
		expect(
			[...names, ...inventory.unavailableSkills.map((skill) => skill.name)].sort(),
			`the host ships every plugin skill to the box and only the prompt is smaller, so a ` +
				`skill in neither list never reached disk`
		).toEqual(theSkillsTheBoxCarries().sort());
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
