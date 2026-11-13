//   monkeys run @production bun run web/scripts/deploy-worker.ts <directory> [--route-subdomain <new label>] [--secret NAME]... [--plan]

import { requiredSetting, setting } from './repository-setting';
import { liveRoutesIn, routePatternOfSubdomain, routesToDeploy, zoneOfSettings, type LiveRoute } from './worker-route';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');
const accountID = requiredSetting('CLOUDFLARE_ACCOUNT_ID');

const given = process.argv.slice(2);
const flags = ['--secret', '--route-subdomain'];
const planFlag = '--plan';

function valuesOf(flag: string): string[] {
	return given.filter((_, index) => given[index - 1] === flag);
}

const secretNames = valuesOf('--secret');
const routeSubdomains = valuesOf('--route-subdomain');
const [directory] = given.filter(
	(argument, index) => argument !== planFlag && !flags.includes(argument) && !flags.includes(given[index - 1] ?? '')
);
if (!directory) throw new Error('name the worker directory to deploy');

const environment = { ...process.env, CLOUDFLARE_API_TOKEN: token, CLOUDFLARE_ACCOUNT_ID: accountID };
const zone = zoneOfSettings({ CLOUDFLARE_DOMAIN: setting('CLOUDFLARE_DOMAIN') });
const namedRoutes = routeSubdomains.map((label) => routePatternOfSubdomain(label, zone));

async function askCloudflare(path: string): Promise<unknown> {
	const response = await fetch(`https://api.cloudflare.com/client/v4${path}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	const body: { success?: boolean; result?: unknown; errors?: unknown } = await response.json();
	if (!body.success) throw new Error(`Cloudflare refused ${path}: ${JSON.stringify(body.errors)}`);
	return body.result;
}

async function workerNameIn(workerDirectory: string): Promise<string> {
	const configuration = Bun.JSONC.parse(await Bun.file(`${workerDirectory}/wrangler.jsonc`).text());
	const name = configuration?.name;
	if (typeof name !== 'string' || !name) throw new Error(`${workerDirectory}/wrangler.jsonc names no worker`);
	return name;
}

async function liveRoutesOf(zoneName: string): Promise<LiveRoute[]> {
	const zones = await askCloudflare(`/zones?name=${encodeURIComponent(zoneName)}`);
	const zoneID = Array.isArray(zones) ? zones[0]?.id : undefined;
	if (typeof zoneID !== 'string') throw new Error(`this account hosts no zone named ${zoneName}`);
	return liveRoutesIn(await askCloudflare(`/zones/${zoneID}/workers/routes`));
}

const routes = routesToDeploy(await liveRoutesOf(zone), await workerNameIn(directory), namedRoutes);

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

if (given.includes(planFlag)) {
	for (const route of routes) console.log(`would deploy to route ${route}`);
	process.exit(0);
}

for (const name of secretNames) {
	const value = setting(name);
	if (!value) throw new Error(`${name} is set neither in this shell nor at the repository root`);
	runWrangler(['secret', 'put', name], value);
}

for (const route of routes) console.log(`deploying to route ${route}`);
runWrangler(['deploy', ...routes.flatMap((route) => ['--route', route])]);
