// Publishes a built SvelteKit app to a Cloudflare Pages project. Every deploy is
// a preview on its own URL unless --production is asked for, so a half-finished
// branch can never land on the live custom domain.
//   bun run web/scripts/deploy-pages.ts --project internkim --output web/.svelte-kit/cloudflare
//   bun run web/scripts/deploy-pages.ts --whoami

import { resolve } from 'node:path';

const token = process.env.CF_API_TOKEN ?? process.env.CLOUDFLARE_API_TOKEN ?? '';
if (!token) throw new Error('set CF_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

function currentBranch(): string {
	const run = Bun.spawnSync(['git', 'rev-parse', '--abbrev-ref', 'HEAD'], { stdout: 'pipe' });
	return new TextDecoder().decode(run.stdout).trim() || 'preview';
}

const isProduction = process.argv.includes('--production');
const project = argument('project');
const outputArgument = argument('output');

if (process.argv.includes('--whoami')) {
	runWrangler(['wrangler', 'whoami']);
}

if (!project || !outputArgument) throw new Error('pass --project <name> --output <path>');

runWrangler([
	'wrangler',
	'pages',
	'deploy',
	resolve(process.cwd(), outputArgument),
	'--project-name',
	project,
	'--branch',
	isProduction ? 'main' : currentBranch(),
	'--commit-dirty=true',
]);

function runWrangler(wranglerArguments: string[]): never {
	const run = Bun.spawnSync(['bunx', ...wranglerArguments], {
		cwd: new URL('..', import.meta.url).pathname,
		env: { ...process.env, CLOUDFLARE_API_TOKEN: token },
		stdout: 'pipe',
		stderr: 'pipe',
	});
	console.log(new TextDecoder().decode(run.stdout));
	const errorOutput = new TextDecoder().decode(run.stderr);
	if (errorOutput.trim()) console.log(errorOutput);
	process.exit(run.exitCode ?? 0);
}
