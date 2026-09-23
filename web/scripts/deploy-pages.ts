//   bun run web/scripts/deploy-pages.ts --project internkim --output web/.svelte-kit/cloudflare
//   bun run web/scripts/deploy-pages.ts --whoami

import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { domainsOf, hostnamesNotAnswering, hostnamesToAnswerFor } from './pages-hostnames';
import { mainCommitOfLiveBuild, refusalToReplaceProduction, stampOfMainCommit } from './production-guard';
import { ensureProductionSchemaIsCurrent } from './production-schema';
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
const accountID = argument('account') ?? requiredSetting('CLOUDFLARE_ACCOUNT_ID');
const project = argument('project');
const outputArgument = argument('output');

if (process.argv.includes('--whoami')) {
	process.exit(runWrangler(['wrangler', 'whoami']));
}

if (!project || !outputArgument)
	throw new Error(
		'pass --project <name> --output <path>; "The company web app" in README.md names the ones this repository deploys'
	);

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
	await ensureProductionSchemaIsCurrent();
}

const output = resolve(process.cwd(), outputArgument);
const deployExitCode = runWrangler([
	'wrangler',
	'pages',
	'deploy',
	output,
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
if (deployExitCode !== 0) process.exit(deployExitCode);
if (isProduction) await waitUntilEveryHostnameAnswersThisBuild();

function runWrangler(wranglerArguments: string[]): number {
	const run = Bun.spawnSync(['bunx', ...wranglerArguments], {
		cwd: fileURLToPath(new URL('..', import.meta.url)),
		env: { ...process.env, CLOUDFLARE_API_TOKEN: token, CLOUDFLARE_ACCOUNT_ID: accountID },
		stdout: 'pipe',
		stderr: 'pipe'
	});
	console.log(new TextDecoder().decode(run.stdout));
	const errorOutput = new TextDecoder().decode(run.stderr);
	if (errorOutput.trim()) console.log(errorOutput);
	return run.exitCode ?? 0;
}

function builtVersion(): string | undefined {
	const versionPath = resolve(output, '_app/version.json');
	if (!existsSync(versionPath)) return undefined;
	const written = JSON.parse(readFileSync(versionPath, 'utf8')) as { version?: unknown };
	if (typeof written.version !== 'string') throw new Error(`${output} carries no build version`);
	return written.version;
}

async function callCloudflare(path: string): Promise<unknown> {
	const response = await fetch(`https://api.cloudflare.com/client/v4${path}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	const body = (await response.json()) as { success: boolean; result?: unknown; errors?: unknown };
	if (!body.success) throw new Error(JSON.stringify(body.errors));
	return body.result;
}

async function waitUntilEveryHostnameAnswersThisBuild(): Promise<void> {
	if (!project) return;
	const version = builtVersion();
	if (version === undefined) {
		console.log(`${output} carries no _app/version.json, so the hostnames of ${project} cannot be checked against this build`);
		return;
	}
	const hostnames = hostnamesToAnswerFor(project, await domainsOf(callCloudflare, accountID, project));
	const attempts = 12;
	for (let attempt = 1; attempt <= attempts; attempt += 1) {
		const behind = await hostnamesNotAnswering(hostnames, version);
		if (behind.length === 0) {
			console.log(`every hostname answers build ${version}: ${hostnames.join(', ')}`);
			return;
		}
		if (attempt < attempts) await Bun.sleep(5000);
		else {
			console.error(
				`${behind.join(', ')} still answer another build than ${version}; something else, such as a Workers route, stands in front of this project there. See web/scripts/pages-domains.ts --project ${project}`
			);
			process.exit(1);
		}
	}
}
