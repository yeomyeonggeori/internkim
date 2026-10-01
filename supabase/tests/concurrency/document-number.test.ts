import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import {
	actAs,
	administratorSession,
	backendProcessID,
	createLoginRole,
	dropLoginRole,
	sessionAs,
	waitUntilWaitingOnLock,
	type LoginRole
} from './racing-sessions';

const firstUserID = '4d000000-0000-0000-0000-000000000001';
const secondUserID = '4d000000-0000-0000-0000-000000000002';
const companyID = '4d000000-0000-0000-0000-0000000000a0';
const prefix = 'INV-2026-';

const racer: LoginRole = {
	name: 'document_number_racer',
	password: crypto.randomUUID(),
	grants: ['usage on schema public', 'execute on function public.reserve_document_number(uuid, text)']
};

const administrator = administratorSession();

async function removeFixtures(): Promise<void> {
	await administrator`delete from public.company where id = ${companyID}`;
	await administrator`delete from auth.users where id in (${firstUserID}, ${secondUserID})`;
	await dropLoginRole(administrator, racer.name);
}

async function insertFixtures(): Promise<void> {
	await administrator`
		insert into auth.users (id, email) values
			(${firstUserID}, 'number-race-first@example.test'),
			(${secondUserID}, 'number-race-second@example.test')
	`;
	await administrator`
		insert into public.company (id, name, slug, country, locale, timezone)
		values (${companyID}, '샘플회사', 'number-race', 'KR', 'ko', 'Asia/Seoul')
	`;
	await administrator`
		insert into public.member (company_id, email, user_id, status, is_admin) values
			(${companyID}, 'number-race-first@example.test', ${firstUserID}, 'active', false),
			(${companyID}, 'number-race-second@example.test', ${secondUserID}, 'active', false)
	`;
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

describe('document number reservation', () => {
	test('a competing reservation waits for the first to commit, then takes the next number', async () => {
		const holder = sessionAs(racer);
		const challenger = sessionAs(racer);
		try {
			await actAs(holder, firstUserID);
			await actAs(challenger, secondUserID);
			await holder`select public.reserve_document_number(${companyID}, ${prefix})`;

			await holder`begin`;
			const [held] = await holder`select public.reserve_document_number(${companyID}, ${prefix}) as number`;

			const challengerProcessID = await backendProcessID(challenger);
			const challenged = challenger`select public.reserve_document_number(${companyID}, ${prefix}) as number`.then((rows) => rows);
			const isChallengerWaiting = await waitUntilWaitingOnLock(administrator, challengerProcessID);
			await holder`commit`;
			const [taken] = await challenged;

			expect({ held: held.number, taken: taken.number }).toEqual({ held: 'INV-2026-002', taken: 'INV-2026-003' });
			expect(isChallengerWaiting).toBe(true);
		} finally {
			await holder.close();
			await challenger.close();
		}
	});
});
