import { expect, test } from 'bun:test';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import {
	theCommandWordsTheEntrypointRuns,
	theProgramsTheEntrypointDeclares,
	whatStarts
} from '../../support/the-entrypoint';

const repositoryRoot = join(import.meta.dir, '..', '..', '..', '..');

test('every program the entrypoint runs is one it checks for', () => {
	const undeclared = theCommandWordsTheEntrypointRuns().filter(
		(program) => !theProgramsTheEntrypointDeclares().includes(program)
	);
	expect(
		undeclared,
		`host/entrypoint.sh runs ${undeclared.join(', ')} without naming it in ` +
			`programsThisScriptRuns, so an image built without it reaches that line and stops there ` +
			`instead of refusing at the door — and the Dockerfile's own --check-programs never asks`
	).toEqual([]);
});

test('the image asks the entrypoint what it needs', () => {
	const dockerfile = readFileSync(join(repositoryRoot, 'host', 'Dockerfile'), 'utf8');
	expect(
		dockerfile,
		`host/Dockerfile no longer runs the entrypoint's own preflight, so a missing binary is ` +
			`found by whoever starts the container rather than by whoever built it`
	).toContain('entrypoint.sh --check-programs');
});

test('the company box tells capabilityd and admind the same messenger', () => {
	expect(
		whatStarts('internkim-admind', '-chatd-platform'),
		`host/entrypoint.sh names a different messenger to each daemon, so a message leaves on ` +
			`whichever one capabilityd was told and answers "sent" either way`
	).toBe(whatStarts('internkim-capabilityd', '--chatd-platform'));
});

test('the company box tells admind and blueclaw the same policy file', () => {
	expect(
		whatStarts('internkim-admind', '-blueclaw-policy'),
		`host/entrypoint.sh has admind reconcile the company roster onto one file and blueclaw read ` +
			`another, so nobody the company hires reaches the agent`
	).toBe(whatStarts('blueclaw', '-policy'));
});

test('the policy admind rewrites is one the box can write', () => {
	expect(
		whatStarts('internkim-admind', '-blueclaw-policy'),
		`admind replaces that file by writing a temp beside it and renaming, so a path under the ` +
			`read-only /etc/blueclaw mount fails every reconcile with EROFS`
	).not.toStartWith('/etc/blueclaw/');
});

test('admind is told where the buzz identity seed lives', () => {
	expect(
		whatStarts('internkim-admind', '-buzz-key-seed-path'),
		`without the seed admind cannot sign a message under a person's own name, and answers as ` +
			`though it had`
	).toBeTruthy();
});

test('the plane template asks for its model through capabilityd', () => {
	const template = JSON.parse(
		readFileSync(join(repositoryRoot, 'host', 'runtime.template.json'), 'utf8')
	) as { languageModel?: { defaultProvider?: string } };
	expect(template.languageModel?.defaultProvider).toBe('capabilityLLM');
});

test('render-company-runtime refuses a template that asks for a model elsewhere', () => {
	const workDirectory = mkdtempSync(join(tmpdir(), 'ikhost-template-'));
	try {
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
			'a plane rendered from that template holds a provider key of its own'
		).not.toBe(0);
		expect(refusal.stderr.toString()).toContain('capabilityLLM');
	} finally {
		rmSync(workDirectory, { recursive: true, force: true });
	}
});
