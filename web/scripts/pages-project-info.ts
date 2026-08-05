//   bun run web/scripts/pages-project-info.ts --project <name>

const token = process.env.CF_API_TOKEN ?? process.env.CLOUDFLARE_API_TOKEN ?? '';
if (!token) throw new Error('set CF_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? '694280310d0ed1189a2a54c4a546403e';
const project = argument('project');
if (!project) throw new Error('pass --project <name>');

type Deployment = {
	id: string;
	environment: string;
	url: string;
	created_on: string;
	deployment_trigger?: { metadata?: { branch?: string } };
};

const headers = { Authorization: `Bearer ${token}` };
const base = `https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${project}`;

const project_response = await fetch(base, { headers });
const projectBody = (await project_response.json()) as {
	success: boolean;
	errors?: unknown;
	result?: { production_branch?: string; subdomain?: string };
};
if (!projectBody.success) throw new Error(JSON.stringify(projectBody.errors));
console.log(`production branch: ${projectBody.result?.production_branch ?? '?'}`);
console.log(`subdomain:         ${projectBody.result?.subdomain ?? '?'}`);

const listed = await fetch(`${base}/deployments?per_page=5`, { headers });
const deploymentBody = (await listed.json()) as { success: boolean; errors?: unknown; result?: Deployment[] };
if (!deploymentBody.success) throw new Error(JSON.stringify(deploymentBody.errors));
console.log('\nrecent deployments:');
for (const deployment of deploymentBody.result ?? []) {
	const branch = deployment.deployment_trigger?.metadata?.branch ?? '?';
	console.log(`  ${deployment.environment.padEnd(11)} ${branch.padEnd(40)} ${deployment.created_on}`);
}
