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

test('the plane template names no model of its own', () => {
	const template = JSON.parse(
		readFileSync(join(repositoryRoot, 'host', 'runtime.template.json'), 'utf8')
	) as { languageModel?: unknown };
	expect(
		template.languageModel,
		'the ladder is rendered from the installed capabilityd, so a copy in the template would drift'
	).toBeUndefined();
});

test('native workspace operations use a helper checked by the image preflight', () => {
	const template = JSON.parse(readFileSync(join(repositoryRoot, 'host', 'runtime.template.json'), 'utf8'));
	expect(template.terminal.mode).toBe('native');
	expect(typeof template.terminal.posixHelperPath).toBe('string');
	expect(theProgramsTheEntrypointDeclares()).toContain(template.terminal.posixHelperPath.split('/').at(-1));
});

test('the entrypoint names the model key file once', () => {
	const entrypoint = readFileSync(join(repositoryRoot, 'host', 'entrypoint.sh'), 'utf8');
	const literalKeyPaths = entrypoint.match(/\/secrets\/openrouter-key/g) ?? [];
	expect(
		literalKeyPaths.length,
		'capabilityd and the rendered ladder read the same key file, so the path is written once'
	).toBe(1);
});

test('the host image follows the current Blueclaw Bun workspace', () => {
	const dockerfile = readFileSync(join(repositoryRoot, 'host', 'Dockerfile'), 'utf8');
	expect(dockerfile).toContain('COPY .dependency/blueclaw/package.json .dependency/blueclaw/bun.lock ./');
	expect(dockerfile).toContain('COPY .dependency/blueclaw/protocol ./protocol');
	expect(dockerfile).toContain('COPY .dependency/blueclaw/chatd ./chatd');
	expect(dockerfile).toContain('COPY .dependency/blueclaw/admin ./admin');
	expect(dockerfile).toContain('COPY host/relay /src/host/relay');
	expect(dockerfile).toContain('bun build --compile --outfile /out/internkim-relay relay.ts');
	expect(dockerfile).toContain('bun build --compile --outfile /out/chatd chatd/src/main.ts');
	expect(dockerfile).toContain('COPY --from=bun-build /out/chatd /usr/local/bin/chatd');
	expect(dockerfile).toContain('COPY --from=bun-build /out/internkim-relay /usr/local/bin/internkim-relay');
	expect(dockerfile).toContain('COPY --from=bun-build /usr/local/bin/bun /usr/local/bin/bun');
	expect(dockerfile).toContain('web/src/lib/i18n/locale.ts');
});

test('the host image does not carry the removed Graphiti service', () => {
	const files = ['host/Dockerfile', 'host/entrypoint.sh', 'host/runtime.template.json', 'tools/render-company-runtime'];
	for (const file of files) {
		expect(readFileSync(join(repositoryRoot, file), 'utf8'), file).not.toContain('graphiti');
	}
});

test('host Buzz account links use one writable path', () => {
	const entrypoint = readFileSync(join(repositoryRoot, 'host', 'entrypoint.sh'), 'utf8');
	expect(entrypoint).toContain('buzzAccountLinksPath="${BUZZ_ACCOUNT_LINKS_PATH:-${CHATD_BUZZ_ACCOUNT_LINKS_PATH:-/var/lib/internkim/buzz-account-links.json}}"');
	expect(whatStarts('internkim-admind', '-buzz-account-links')).toContain('/var/lib/internkim/buzz-account-links.json');
	expect(entrypoint).toContain('CHATD_BUZZ_ACCOUNT_LINKS_PATH="${buzzAccountLinksPath}"');
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
