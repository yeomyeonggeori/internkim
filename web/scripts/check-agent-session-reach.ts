import { controlPlane, sessionForPlatformIdentity } from '../src/lib/server/control-plane';
import { claimABoxWithItsSession, releaseTheClaimedBox } from '../tests/support/a-claimed-box';

const projectURL = process.env.SUPABASE_URL ?? '';
const publishableKey = process.env.SUPABASE_PUBLISHABLE_KEY ?? '';
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? '';
const signingKey = process.env.SUPABASE_JWT_SIGNING_KEY ?? '';
if (!projectURL || !publishableKey || !serviceRoleKey || !signingKey) {
	throw new Error('needs SUPABASE_URL, SUPABASE_PUBLISHABLE_KEY, SUPABASE_SECRET_KEY and SUPABASE_JWT_SIGNING_KEY');
}
const credentials = { projectURL, publishableKey, serviceRoleKey, signingKey };

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const platform = argument('platform') ?? 'mattermost';
const wantedSlug = argument('company');
const client = controlPlane(credentials);

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

const box = await claimABoxWithItsSession(credentials, company.id);

let reached = 0;
for (const member of members) {
	const externalID = member.messenger?.[platform];
	if (!externalID) {
		console.log(`${member.email}: no ${platform} account on the member`);
		continue;
	}
	try {
		const session = await sessionForPlatformIdentity(credentials, box.session.accessToken, platform, externalID);
		reached += 1;
		console.log(`${member.email}: session ok (${session.memberID})`);
	} catch (errorValue) {
		console.log(`${member.email}: REFUSED ${(errorValue as Error).message}`);
	}
}
await releaseTheClaimedBox(client, company.id, box);

console.log(`\n${reached}/${members.length} members of ${company.slug} the company computer can act for`);
