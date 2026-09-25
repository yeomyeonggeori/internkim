import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import {
	actAs,
	administratorSession,
	backendProcessID,
	createLoginRole,
	dropLoginRole,
	refusalOf,
	sessionAs,
	waitUntilWaitingOnLock,
	type LoginRole
} from './racing-sessions';

const userID = '4d000000-0000-0000-0000-000000000001';
const companyID = '4d000000-0000-0000-0000-0000000000a0';
const memberID = '4d000000-0000-0000-0000-0000000000a1';
const winningParentID = '4d000000-0000-0000-0000-000000000101';
const challengingParentID = '4d000000-0000-0000-0000-000000000102';
const contendedChildID = '4d000000-0000-0000-0000-000000000103';

const racer: LoginRole = {
	name: 'task_child_racer',
	password: crypto.randomUUID(),
	grants: [
		'usage on schema public',
		'execute on function public.task_children_link(uuid, uuid[])',
		'execute on function public.task_parent_set(uuid, uuid)'
	]
};

const administrator = administratorSession();

async function removeFixtures(): Promise<void> {
	await administrator`delete from public.task_participant where member_id = ${memberID}`;
	await administrator`delete from public.task where company_id = ${companyID}`;
	await administrator`delete from public.member where id = ${memberID}`;
	await administrator`delete from public.company where id = ${companyID}`;
	await administrator`delete from auth.users where id = ${userID}`;
	await dropLoginRole(administrator, racer.name);
}

async function insertFixtures(): Promise<void> {
	await administrator`insert into auth.users (id, email) values (${userID}, 'race-link@example.test')`;
	await administrator`
		insert into public.company (id, name, slug, country, locale, timezone)
		values (${companyID}, 'Race Link', 'race-link', 'KR', 'ko', 'Asia/Seoul')
	`;
	await administrator`
		insert into public.member (id, company_id, email, user_id, status)
		values (${memberID}, ${companyID}, 'race-link@example.test', ${userID}, 'active')
	`;
	await administrator`
		insert into public.task (id, company_id, title, parent_task_id) values
			(${winningParentID}, ${companyID}, 'Winning parent', null),
			(${challengingParentID}, ${companyID}, 'Challenging parent', null),
			(${contendedChildID}, ${companyID}, 'Contended child', null)
	`;
	await administrator`
		insert into public.task_participant (task_id, member_id) values
			(${winningParentID}, ${memberID}),
			(${challengingParentID}, ${memberID}),
			(${contendedChildID}, ${memberID})
	`;
}

async function contendedChildParentID(): Promise<string | null> {
	const rows: { parent_task_id: string | null }[] = await administrator`
		select parent_task_id from public.task where id = ${contendedChildID}
	`;
	return rows[0]?.parent_task_id ?? null;
}

beforeAll(async () => {
	await removeFixtures();
	await createLoginRole(administrator, racer);
	await insertFixtures();
});

afterAll(async () => {
	await removeFixtures();
	await administrator.close();
});

describe('linking task children', () => {
	test('a competing request reaches the child row lock and cannot overwrite the first committed parent', async () => {
		const holder = sessionAs(racer);
		const challenger = sessionAs(racer);
		try {
			await actAs(holder, userID);
			await actAs(challenger, userID);
			await holder`begin`;
			await holder`select public.task_parent_set(${contendedChildID}::uuid, ${winningParentID}::uuid)`;

			const challengerProcessID = await backendProcessID(challenger);
			const challengeRefusal = refusalOf(
				challenger`select public.task_children_link(${challengingParentID}::uuid, array[${contendedChildID}]::uuid[])`
			);
			const isChallengerWaiting = await waitUntilWaitingOnLock(administrator, challengerProcessID);
			await holder`commit`;

			expect(isChallengerWaiting).toBe(true);
			expect(await challengeRefusal).toBe(
				'every selected child task must be available to the authenticated member and have no parent'
			);
			expect(await contendedChildParentID()).toBe(winningParentID);
		} finally {
			await holder.close();
			await challenger.close();
		}
	});
});
