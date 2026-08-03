// Publishes a built SvelteKit app to a Cloudflare Pages project. Reads the
// Cloudflare token from .env the same way the other scripts read credentials.
//   bun run web/scripts/deploy-pages.ts --project <name> --output <path to .svelte-kit/cloudflare>
//   bun run web/scripts/deploy-pages.ts --whoami

const token = process.env.CF_API_TOKEN ?? process.env.CLOUDFLARE_API_TOKEN ?? '';
if (!token) throw new Error('set CF_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const project = argument('project');
const output = argument('output');
const wranglerArguments = process.argv.includes('--whoami')
	? ['wrangler', 'whoami']
	: ['wrangler', 'pages', 'deploy', output ?? '.svelte-kit/cloudflare', '--project-name', project ?? '', '--branch', 'main', '--commit-dirty=true'];

if (!process.argv.includes('--whoami') && (!project || !output)) {
	throw new Error('pass --project <name> --output <path>');
}

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
