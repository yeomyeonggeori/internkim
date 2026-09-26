import { expect, test } from 'bun:test';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const repositoryRoot = join(import.meta.dir, '..', '..', '..', '..');

test('the plane template names no model of its own', () => {
	const template = JSON.parse(
		readFileSync(join(repositoryRoot, 'host', 'runtime.template.json'), 'utf8')
	) as { languageModel?: unknown };
	expect(
		template.languageModel,
		'the ladder is rendered from the installed capabilityd, so a copy in the template would drift'
	).toBeUndefined();
});

test('render-company-runtime refuses a template carrying its own ladder', () => {
	const workDirectory = mkdtempSync(join(tmpdir(), 'ikhost-template-'));
	try {
		const templatePath = join(workDirectory, 'runtime.template.json');
		writeFileSync(
			templatePath,
			JSON.stringify({
				languageModel: {
					tiers: {
						low: [{ endpoint: 'https://models.example.com/v1', model: 'example/model' }]
					}
				}
			})
		);

		const refusal = Bun.spawnSync(
			[
				join(repositoryRoot, 'tools', 'render-company-runtime'),
				'--template',
				templatePath,
				'--out',
				join(workDirectory, 'runtime.json')
			],
			{
				env: {
					...process.env,
					DATABASE_URL: 'postgres://nobody/nothing',
					MESSENGER_PLATFORM: 'buzz'
				}
			}
		);

		expect(
			refusal.exitCode,
			'a template that carries a ladder is a second copy of what capabilityd answers for'
		).not.toBe(0);
		expect(refusal.stderr.toString()).toContain('languageModel');
	} finally {
		rmSync(workDirectory, { recursive: true, force: true });
	}
});
