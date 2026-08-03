// Lists a company's agents: what is running, what was retired, when each last called.
//   bun run web/scripts/show-agents.ts [--check <api key>]

import { agentOfKey, controlPlane } from '../src/lib/server/control-plane';

const index = process.argv.indexOf('--check');
const apiKey = index >= 0 ? process.argv[index + 1] : undefined;

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: agents, error } = await client
	.from('agent')
	.select('id, company_id, name, created_at, last_seen_at, revoked_at')
	.order('created_at');
if (error) throw new Error(error.message);

for (const agent of agents ?? []) {
	const state = agent.revoked_at ? 'retired' : 'live';
	const seen = agent.last_seen_at ?? 'never';
	console.log(`  ${agent.name.padEnd(12)} ${state.padEnd(8)} last seen ${seen}`);
}
if ((agents ?? []).length === 0) console.log('  no agents');

if (apiKey) {
	const found = await agentOfKey(client, apiKey);
	console.log(`\nthat key: ${found ? `agent ${found.agentID}` : 'belongs to no live agent'}`);
}
