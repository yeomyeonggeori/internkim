import { afterAll, beforeAll, expect, test } from 'bun:test';
import { addMember, claimMemberFor } from '../../src/lib/server/control-plane';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';

// Somebody is invited and asks the agent something in the same minute. The
// roster is what decides who may act at all, and the plane reads it again when
// the company says its directory changed — so "just invited" and "can be
// written to" are the same moment, not two minutes apart.

let plane: ACompanyPlane;

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz' });
}, 120_000);

afterAll(async () => {
	await plane?.stop();
});

async function peopleTheDeviceKnows(): Promise<string[]> {
	const answer = await fetch(`${plane.blueclawURL}/admin/api/policy`);
	const policy = (await answer.json()) as { people?: { emails?: string[] }[] };
	return (policy.people ?? []).flatMap((person) => person.emails ?? []);
}

test('somebody invited after the plane is up reaches it when the company says so', async () => {
	const newcomerEmail = `newcomer-${crypto.randomUUID().slice(0, 8)}@example.test`;
	expect(await peopleTheDeviceKnows()).not.toContain(newcomerEmail);

	const memberID = await addMember(plane.admin, plane.companyID, newcomerEmail);
	expect(memberID).toBeTruthy();
	const account = await plane.admin.auth.admin.createUser({
		email: newcomerEmail,
		password: 'seed-password',
		email_confirm: true
	});
	if (account.error) throw new Error(account.error.message);
	await claimMemberFor(plane.admin, account.data.user.id, newcomerEmail);

	const told = await fetch(`${plane.admindURL}/admin/api/directory/changed`, { method: 'POST' });
	expect(told.status).toBe(202);

	// One read, no polling: the door is synchronous, and a door that answers 202
	// before it has done the work is the defect this pins.
	expect(
		await peopleTheDeviceKnows(),
		'the company said its directory changed and the device did not read the roster again, ' +
			'so somebody just invited cannot be resolved until the two-minute pass comes round'
	).toContain(newcomerEmail);
});
