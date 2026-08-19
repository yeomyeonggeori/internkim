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
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
if (!projectURL || !serviceRoleKey) throw new Error('needs SUPABASE_URL and SUPABASE_SECRET_KEY');

const platform = process.argv[2] ?? 'mattermost';
const client = controlPlane({ projectURL, serviceRoleKey });

const { data: company, error: companyError } = await client
	.from('company')
	.select('id')
	.limit(1)
	.single();
if (companyError) throw new Error(companyError.message);

const issued = await issueAgentKey(client, company.id, 'session reach check');

const { data: members, error: memberError } = await client
	.from('member')
	.select('email, messenger')
	.order('email')
	.returns<{ email: string; messenger: Record<string, string> | null }[]>();
if (memberError) throw new Error(memberError.message);

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
console.log(`\n${reached}/${members.length} members the device can act for`);
