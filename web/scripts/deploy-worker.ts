//   bun run web/scripts/deploy-worker.ts <directory> [--route-subdomain <label>] [--secret NAME]...

import { requiredSetting, setting } from './repository-setting';
import { routePatternOfSubdomain, zoneOfSettings } from './worker-route';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');
const accountID = requiredSetting('CLOUDFLARE_ACCOUNT_ID');

const given = process.argv.slice(2);
const flags = ['--secret', '--route-subdomain'];

function valuesOf(flag: string): string[] {
	return given.filter((_, index) => given[index - 1] === flag);
}

const secretNames = valuesOf('--secret');
const routeSubdomains = valuesOf('--route-subdomain');
const [directory] = given.filter(
	(argument, index) => !flags.includes(argument) && !flags.includes(given[index - 1] ?? '')
);
if (!directory) throw new Error('name the worker directory to deploy');

const environment = { ...process.env, CLOUDFLARE_API_TOKEN: token, CLOUDFLARE_ACCOUNT_ID: accountID };
const zone = zoneOfSettings({ CLOUDFLARE_DOMAIN: setting('CLOUDFLARE_DOMAIN'), INTERNKIM_DOMAIN: setting('INTERNKIM_DOMAIN') });
const routes = routeSubdomains.map((label) => routePatternOfSubdomain(label, zone));

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

for (const route of routes) console.log(`deploying to route ${route}`);
runWrangler(['deploy', ...routes.flatMap((route) => ['--route', route])]);
