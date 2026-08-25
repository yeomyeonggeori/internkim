//   bun run web/scripts/deploy-pages.ts --project internkim --output web/.svelte-kit/cloudflare
//   bun run web/scripts/deploy-pages.ts --whoami

import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mainCommitOfLiveBuild, refusalToReplaceProduction, stampOfMainCommit } from './production-guard';
import { requiredSetting } from './repository-setting';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

function runGit(...gitArguments: string[]): { output: string; succeeded: boolean } {
	const run = Bun.spawnSync(['git', ...gitArguments], { stdout: 'pipe', stderr: 'pipe' });
	return { output: new TextDecoder().decode(run.stdout).trim(), succeeded: run.exitCode === 0 };
}

function currentBranch(): string {
	return runGit('rev-parse', '--abbrev-ref', 'HEAD').output || 'preview';
}

function treeHas(commit: string): boolean {
	return runGit('merge-base', '--is-ancestor', commit, 'HEAD').succeeded;
}

async function readLiveBuildMessage(accountID: string, projectName: string): Promise<string | null> {
	const response = await fetch(`https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${projectName}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	const body = (await response.json()) as {
		success: boolean;
		result?: { canonical_deployment?: { deployment_trigger?: { metadata?: { commit_message?: string } } } };
	};
	if (!body.success) return null;
	return body.result?.canonical_deployment?.deployment_trigger?.metadata?.commit_message ?? null;
}

const isProduction = process.argv.includes('--production');
const isReplacingNewerAllowed = process.argv.includes('--replace-newer');
const accountID = argument('account') ?? '694280310d0ed1189a2a54c4a546403e';
const project = argument('project');
const outputArgument = argument('output');

if (process.argv.includes('--whoami')) {
	runWrangler(['wrangler', 'whoami']);
}

if (!project || !outputArgument) throw new Error('pass --project <name> --output <path>');

if (isProduction) {
	runGit('fetch', '--quiet', 'origin', 'main');
	const liveMainCommit = mainCommitOfLiveBuild(await readLiveBuildMessage(accountID, project));
	const refusal = refusalToReplaceProduction({
		containsOriginMain: treeHas('origin/main'),
		liveMainCommit,
		treeHasLiveMainCommit: liveMainCommit === null ? true : treeHas(liveMainCommit),
		isReplacingNewerAllowed
	});
	if (refusal) {
		console.error(`refusing to replace production: ${refusal}`);
		process.exit(1);
	}
}

runWrangler([
	'wrangler',
	'pages',
	'deploy',
	resolve(process.cwd(), outputArgument),
	'--project-name',
	project,
	'--branch',
	isProduction ? 'main' : currentBranch(),
	'--commit-hash',
	runGit('rev-parse', 'HEAD').output,
	'--commit-message',
	stampOfMainCommit(runGit('rev-parse', 'origin/main').output),
	'--commit-dirty=true'
]);

function runWrangler(wranglerArguments: string[]): never {
	const run = Bun.spawnSync(['bunx', ...wranglerArguments], {
		cwd: fileURLToPath(new URL('..', import.meta.url)),
		env: { ...process.env, CLOUDFLARE_API_TOKEN: token, CLOUDFLARE_ACCOUNT_ID: accountID },
		stdout: 'pipe',
		stderr: 'pipe'
	});
	console.log(new TextDecoder().decode(run.stdout));
	const errorOutput = new TextDecoder().decode(run.stderr);
	if (errorOutput.trim()) console.log(errorOutput);
	process.exit(run.exitCode ?? 0);
}
