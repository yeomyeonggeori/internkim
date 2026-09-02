import { expect, test } from 'bun:test';
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { theProgramsTheEntrypointRuns } from './the-entrypoint';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');

test('the image carries every program its entrypoint runs', () => {
	const dockerfile = readFileSync(join(repositoryRoot, 'host', 'Dockerfile'), 'utf8');
	const missing = theProgramsTheEntrypointRuns().filter(
		(program) => !dockerfile.includes(`/usr/local/bin/${program}`)
	);
	expect(
		missing,
		`host/Dockerfile installs no ${missing.join(', ')}, so the image cannot run its own ` +
			`entrypoint: docker compose up brings the bundle to whichever daemon comes first and stops`
	).toEqual([]);
});

test('render-company-runtime refuses a template that asks for a model elsewhere', () => {
	const workDirectory = mkdtempSync(join(tmpdir(), 'ikplane-template-'));
	const templatePath = join(workDirectory, 'runtime.template.json');
	writeFileSync(
		templatePath,
		JSON.stringify({
			languageModel: {
				defaultProvider: 'direct',
				direct: { endpoint: 'https://openrouter.ai/api/v1', apiKeyPath: '/secrets/openrouter-key' }
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
		{ env: { ...process.env, DATABASE_URL: 'postgres://nobody/nothing', MESSENGER_PLATFORM: 'buzz' } }
	);

	expect(refusal.exitCode, 'a plane rendered from that template holds a provider key of its own').not.toBe(0);
	expect(refusal.stderr.toString()).toContain('capabilityLLM');
});

test('the plane template asks for its model through capabilityd', () => {
	const template = JSON.parse(
		readFileSync(join(repositoryRoot, 'host', 'runtime.template.json'), 'utf8')
	) as { languageModel?: { defaultProvider?: string } };
	expect(template.languageModel?.defaultProvider).toBe('capabilityLLM');
});
