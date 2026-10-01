import {
	controlPlane,
	issueAgentKey,
	revokeAgent,
	sessionForPlatformIdentity
} from '../src/lib/server/control-plane';

// The device asks for a member session before it can write anything as that
// person. This reports who it can reach, because a resolver reading the wrong
// table answers for one member and refuses the rest without anyone noticing.

const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? '';
if (!projectURL || !serviceRoleKey) throw new Error('needs SUPABASE_URL and SUPABASE_SECRET_KEY');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const platform = argument('platform') ?? 'mattermost';
const wantedSlug = argument('company');
const client = controlPlane({ projectURL, serviceRoleKey });

type CompanyRow = { id: string; name: string; slug: string };

const { data: companies, error: companyError } = await client
	.from('company')
	.select('id, name, slug')
	.order('slug')
	.returns<CompanyRow[]>();
if (companyError) throw new Error(companyError.message);

function companyToAskAbout(): CompanyRow | undefined {
	if (wantedSlug) return companies.find((row) => row.slug === wantedSlug);
	if (companies.length === 1) return companies[0];
	return undefined;
}

const company = companyToAskAbout();
if (!company) {
	console.log('name the company with --company <slug>. This project holds:');
	for (const row of companies) console.log(`  ${row.slug}  ${row.name}`);
	process.exit(1);
}

const { data: members, error: memberError } = await client
	.from('member')
	.select('email, messenger')
	.eq('company_id', company.id)
	.order('email')
	.returns<{ email: string; messenger: Record<string, string> | null }[]>();
if (memberError) throw new Error(memberError.message);

// Revoking keeps the row and the row keeps the name, which is unique per
// company, so a key made only to ask this question is revoked and then deleted:
// a run that dies between the two leaves a dead key rather than a live one.
const agentName = 'session reach check';
const cleared = await client.from('agent').delete().eq('company_id', company.id).eq('name', agentName);
if (cleared.error) throw new Error(`clearing ${agentName}: ${cleared.error.message}`);

const issued = await issueAgentKey(client, company.id, agentName);

let reached = 0;
for (const member of members) {
	const externalID = member.messenger?.[platform];
	if (!externalID) {
		console.log(`${member.email}: no ${platform} account on the member`);
		continue;
	}
	try {
		const session = await sessionForPlatformIdentity(
			{ projectURL, serviceRoleKey },
			issued.apiKey,
			platform,
			externalID
		);
		reached += 1;
		console.log(`${member.email}: session ok (${session.memberID})`);
	} catch (errorValue) {
		console.log(`${member.email}: REFUSED ${(errorValue as Error).message}`);
	}
}
await revokeAgent(client, issued.agentID);
const removed = await client.from('agent').delete().eq('id', issued.agentID);
if (removed.error) throw new Error(`removing ${agentName}: ${removed.error.message}`);

console.log(`\n${reached}/${members.length} members of ${company.slug} the device can act for`);
