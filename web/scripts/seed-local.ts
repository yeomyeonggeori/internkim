//   bun run web/scripts/seed-local.ts

import { addMember, inviteMember, controlPlane, provisionCompany } from '../src/lib/server/control-plane';

const projectURL = process.env.SUPABASE_URL ?? '';
if (!projectURL.includes('127.0.0.1') && !projectURL.includes('localhost')) {
	throw new Error(`this only seeds a local stack, and SUPABASE_URL is ${projectURL}`);
}

const client = controlPlane({
	projectURL,
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const people = [
	{ name: '이샘플', email: 'lee@example.com', jobTitle: 'CTO', isAdmin: true },
	{ name: '김표본', email: 'iam@example.com', jobTitle: 'CEO', isAdmin: true },
	{ name: '박예시', email: 'gyeonyang@example.com', jobTitle: '연구원', isAdmin: false },
];

const { companyID } = await provisionCompany(
	client,
	{
		name: '여명거리',
		slug: 'dawnstreet',
		country: 'KR',
		locale: 'ko',
		timezone: 'Asia/Seoul',
		workLocations: [
			{ name: '사무실', color: '#9929bd' },
			{ name: '재택', color: '#669c35' },
			{ name: '외부', color: '#0ea5e9' },
		],
	},
	people[0].email,
);

await client
	.from('company')
	.update({
		task_vocabulary: {
			businesses: [{ name: '여명거리', color: '#216fe4' }, { name: '오토케', color: '#475569' }],
			types: [{ name: '기능' }, { name: '개선' }, { name: '회의' }],
		},
	})
	.eq('id', companyID);

for (const person of people) {
	const memberID = await addMember(client, companyID, person.email, { isAdmin: person.isAdmin });
	const invitation = await inviteMember(client, memberID);
	await client
		.from('member')
		.update({ name: person.name, job_title: person.jobTitle, status: 'active', joined_at: '2026-01-02T00:00:00Z' })
		.eq('id', memberID);
	console.log(`${person.name.padEnd(6)} ${person.email.padEnd(18)} ${invitation.temporaryPassword}`);
}

console.log(`\ncompany ${companyID}`);
