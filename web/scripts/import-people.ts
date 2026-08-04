// Imports a device roster into a company. Idempotent: rerunning changes nothing.
// It never sends email — accounts are created silently and members stay `pending`
// until somebody actually invites them.
//
//   internkim users list | bun run scripts/import-people.ts --company <uuid>
//   ... --create-company "Name" --slug name --country KR --locale ko --timezone Asia/Seoul

import { addMember, controlPlane, provisionCompany } from '../src/lib/server/control-plane';

type Person = { email: string; role: string; handle: string; name: string };

function parseRoster(text: string): Person[] {
	return text
		.split('\n')
		.map((line) => line.trim())
		.filter((line) => line && !line.startsWith('EMAIL'))
		.map((line) => line.split(/\s{2,}/).map((field) => field.trim()))
		.filter((fields) => fields.length >= 2 && fields[0].includes('@'))
		.map((fields) => ({
			email: fields[0],
			role: fields[1] ?? 'member',
			handle: fields[2] ?? '',
			name: fields[3] ?? '',
		}));
}

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const roster = parseRoster(await Bun.stdin.text());
if (roster.length === 0) throw new Error('no people on stdin');

let companyID = argument('company') ?? '';
const companyName = argument('create-company');
if (!companyID && companyName) {
	const provisioned = await provisionCompany(
		client,
		{
			name: companyName,
			slug: argument('slug') ?? companyName.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
			country: argument('country') ?? 'KR',
			locale: argument('locale') ?? 'ko',
			timezone: argument('timezone') ?? 'Asia/Seoul',
			workLocations: argument('work-locations')?.split(',').filter(Boolean).map((name) => ({ name })),
		},
		roster.find((person) => person.role === 'admin')?.email ?? roster[0].email,
	);
	companyID = provisioned.companyID;
	console.log(`created company ${companyID}`);
}
if (!companyID) throw new Error('pass --company <uuid> or --create-company <name>');

const { data: existing } = await client.auth.admin.listUsers();
const accountByEmail = new Map(existing.users.map((user) => [user.email ?? '', user.id]));

for (const person of roster) {
	// The member has to exist first: the bind trigger looks for a matching member
	// when the account appears, so creating the account first leaves them unlinked.
	const memberID = await addMember(client, companyID, person.email, {
		isAdmin: person.role === 'admin',
	});

	let accountID = accountByEmail.get(person.email);
	if (!accountID) {
		const { data, error } = await client.auth.admin.createUser({
			email: person.email,
			email_confirm: false,
			user_metadata: { full_name: person.name, handle: person.handle },
		});
		if (error) throw new Error(`account ${person.email}: ${error.message}`);
		accountID = data.user.id;
	}

	// Repairs a member left unlinked by an earlier run that created the account first.
	const { error: bindError } = await client
		.from('member')
		.update({ user_id: accountID })
		.eq('id', memberID)
		.is('user_id', null);
	if (bindError) throw new Error(`bind ${person.email}: ${bindError.message}`);

	console.log(`${person.email.padEnd(30)} ${person.role.padEnd(7)} member ${memberID}`);
}

const { count } = await client
	.from('member')
	.select('*', { count: 'exact', head: true })
	.eq('company_id', companyID);
console.log(`company ${companyID} now has ${count} members`);
