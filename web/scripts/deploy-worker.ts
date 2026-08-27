//   bun run web/scripts/deploy-worker.ts <directory> [--secret NAME=value]...

import { requiredSetting, setting } from './repository-setting';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');
const accountID = '694280310d0ed1189a2a54c4a546403e';

const given = process.argv.slice(2);
const secretNames = given.filter((_, index) => given[index - 1] === '--secret');
const [directory] = given.filter(
	(argument, index) => argument !== '--secret' && given[index - 1] !== '--secret'
);
if (!directory) throw new Error('name the worker directory to deploy');

const environment = { ...process.env, CLOUDFLARE_API_TOKEN: token, CLOUDFLARE_ACCOUNT_ID: accountID };

function runWrangler(wranglerArguments: string[], input?: string): void {
	const run = Bun.spawnSync(['bunx', 'wrangler', ...wranglerArguments], {
		cwd: directory,
		env: environment,
		stdin: input === undefined ? 'inherit' : new TextEncoder().encode(input),
		stdout: 'inherit',
		stderr: 'inherit'
	});
	if (run.exitCode !== 0) process.exit(run.exitCode ?? 1);
}

for (const name of secretNames) {
	const value = setting(name);
	if (!value) throw new Error(`${name} is set neither in this shell nor at the repository root`);
	runWrangler(['secret', 'put', name], value);
}

runWrangler(['deploy']);
