import { afterAll, beforeAll, describe, expect, spyOn, test } from 'bun:test';
import { x25519 } from '@noble/curves/ed25519.js';
import { z } from 'zod';
import { sealedSecretSchema, type SealedSecret } from '../../src/lib/company/box';
import { base64URLOf } from '../../src/lib/company/seal-to-box';
import {
	addMember,
	claimFleetForCompany,
	controlPlane,
	issueAgentKey,
	provisionCompany,
	sessionForMember
} from '../../src/lib/server/control-plane';
import { mailPasswordPurpose } from '../../src/lib/server/mail-account';
import { keepMemberCredential } from '../../src/lib/server/member-credential';
import { mailAccountCredentialKind } from '../../src/lib/server/public-api/catalog/credential';
import { resealMailAccounts } from '../../scripts/reseal-mail-accounts';
import { openedBy } from '../unit/company/open-from-box';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

const { GET: readOwnAccount, PUT: writeOwnAccount } = await import('../../src/routes/api/member/mail-account/+server');
const { GET: readAccountAsHost } = await import('../../src/routes/api/agent/mail-account/+server');
const { GET: readAccountsAsHost } = await import('../../src/routes/api/agent/mail-accounts/+server');

const networkHookTimeout = 60_000;
const credentials = { projectURL, serviceRoleKey, signingKey };
const client = controlPlane(credentials);
const stamp = Date.now();
const environment = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};
const boxSecretKey = x25519.utils.randomSecretKey();
const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));
const companyIDs: string[] = [];

const answeredAccount = z.object({ account: z.record(z.string(), z.unknown()) });
const answeredAccounts = z.object({ accounts: z.array(z.record(z.string(), z.unknown())) });

type Company = { companyID: string; memberID: string; memberToken: string; agentKey: string };

let boxed: Company;
let frozen: Company;

const written = {
	email: `mail-${stamp}@example.test`,
	imapHost: 'imap.example.test',
	imapUsername: 'sample',
	imapPassword: `imap-password-${stamp}`,
	smtpHost: 'smtp.example.test',
	smtpUsername: 'sample',
	smtpPassword: `smtp-password-${stamp}`
};

async function aCompany(label: string): Promise<Company> {
	const slug = `mail-${label}-${stamp}`;
	const provisioned = await provisionCompany(
		client,
		{ name: `Mail ${label}`, slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyIDs.push(provisioned.companyID);
	const memberID = await addMember(client, provisioned.companyID, `${slug}-member@example.test`);
	const account = await client.auth.admin.createUser({ email: `${slug}-member@example.test`, email_confirm: true });
	if (account.error) throw new Error(account.error.message);
	await client.from('member').update({ status: 'active', user_id: account.data.user.id }).eq('id', memberID);
	return {
		companyID: provisioned.companyID,
		memberID,
		memberToken: (await sessionForMember(credentials, memberID)).accessToken,
		agentKey: (await issueAgentKey(client, provisioned.companyID, 'mail')).apiKey
	};
}

function requestBearing(token: string, path: string, init: RequestInit = {}): Request {
	return new Request(`https://intern.example.test${path}`, {
		...init,
		headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' }
	});
}

async function writeAs(company: Company, body: Record<string, unknown>): Promise<Record<string, unknown>> {
	const request = requestBearing(company.memberToken, '/api/member/mail-account', {
		method: 'PUT',
		body: JSON.stringify(body)
	});
	const response = await writeOwnAccount({ request, platform: { env: environment } } as unknown as Parameters<
		typeof writeOwnAccount
	>[0]);
	return answeredAccount.parse(await response.json()).account;
}

async function readAs(company: Company): Promise<Record<string, unknown>> {
	const request = requestBearing(company.memberToken, '/api/member/mail-account');
	const response = await readOwnAccount({ request, platform: { env: environment } } as unknown as Parameters<
		typeof readOwnAccount
	>[0]);
	return answeredAccount.parse(await response.json()).account;
}

async function readAsHost(company: Company): Promise<Record<string, unknown>> {
	const path = `/api/agent/mail-account?memberID=${company.memberID}`;
	const request = requestBearing(company.agentKey, path);
	const response = await readAccountAsHost({
		request,
		url: new URL(request.url),
		platform: { env: environment }
	} as unknown as Parameters<typeof readAccountAsHost>[0]);
	return answeredAccount.parse(await response.json()).account;
}

async function listAsHost(company: Company): Promise<Record<string, unknown>[]> {
	const request = requestBearing(company.agentKey, '/api/agent/mail-accounts');
	const response = await readAccountsAsHost({ request, platform: { env: environment } } as unknown as Parameters<
		typeof readAccountsAsHost
	>[0]);
	return answeredAccounts.parse(await response.json()).accounts;
}

async function storedSecretOf(memberID: string): Promise<string> {
	const { data, error } = await client.rpc('read_member_secret', {
		target_member: memberID,
		target_kind: mailAccountCredentialKind
	});
	if (error) throw new Error(error.message);
	return data;
}

function sealedOf(offered: unknown): SealedSecret {
	return sealedSecretSchema.parse(offered);
}

beforeAll(async () => {
	boxed = await aCompany('boxed');
	await claimFleetForCompany(client, boxed.companyID, base64URLOf(x25519.utils.randomSecretKey()), {
		encryptionKey: boxEncryptionKey
	});
	frozen = await aCompany('frozen');
}, networkHookTimeout);

afterAll(async () => {
	for (const companyID of companyIDs) {
		const { data: members } = await client.from('member').select('id, user_id').eq('company_id', companyID);
		for (const member of members ?? []) {
			await client.from('credential').delete().eq('member_id', member.id);
		}
		await client.from('company').delete().eq('id', companyID);
		for (const member of members ?? []) {
			if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
		}
	}
}, networkHookTimeout);

describe('a company with a box keeps its mail passwords sealed to it', () => {
	test('what Vault holds names no password in the clear, and the box opens it for that member alone', async () => {
		await writeAs(boxed, written);

		const stored = await storedSecretOf(boxed.memberID);
		const held = JSON.parse(stored);

		expect(stored.includes(written.imapPassword)).toBe(false);
		expect(stored.includes(written.smtpPassword)).toBe(false);
		expect(held.IMAPPassword).toBe('');
		expect(held.SMTPPassword).toBe('');
		expect(held.IMAPHost).toBe(written.imapHost);
		const imapPassword = sealedOf(held.SealedIMAPPassword);
		expect(imapPassword.recipient).toBe(boxEncryptionKey);
		const owner = { companyID: boxed.companyID, memberID: boxed.memberID };
		expect(await openedBy(boxSecretKey, imapPassword, mailPasswordPurpose(owner, 'IMAPPassword'))).toBe(
			written.imapPassword
		);
		await expect(
			openedBy(boxSecretKey, imapPassword, mailPasswordPurpose({ ...owner, memberID: frozen.memberID }, 'IMAPPassword'))
		).rejects.toThrow();
	});

	test('saving with the password left blank keeps the sealed one byte for byte', async () => {
		const before = JSON.parse(await storedSecretOf(boxed.memberID));
		expect(sealedOf(before.SealedIMAPPassword).recipient).toBe(boxEncryptionKey);

		await writeAs(boxed, { ...written, imapHost: 'imap2.example.test', imapPassword: '', smtpPassword: '' });

		const after = JSON.parse(await storedSecretOf(boxed.memberID));
		expect(after.SealedIMAPPassword).toEqual(before.SealedIMAPPassword);
		expect(after.SealedSMTPPassword).toEqual(before.SealedSMTPPassword);
		expect(after.IMAPHost).toBe('imap2.example.test');
	});

	test('the mail tab is shown the account as configured and no password', async () => {
		const shown = await readAs(boxed);

		expect(shown).toMatchObject({ isConfigured: true, hasIMAPPassword: true, hasSMTPPassword: true });
		expect(JSON.stringify(shown).includes('ciphertext')).toBe(false);
	});

	test('the company computer is handed the sealed passwords and whose they are', async () => {
		const account = await readAsHost(boxed);
		const listed = await listAsHost(boxed);

		expect(account.IMAPPassword).toBe('');
		expect(sealedOf(account.SealedIMAPPassword).recipient).toBe(boxEncryptionKey);
		expect(listed.map((held) => held.MemberID)).toEqual([boxed.memberID]);
	});
});

describe('a company with no box key, on the frozen device path', () => {
	test('keeps the readable form and says so, naming the company', async () => {
		const said = spyOn(console, 'error').mockImplementation(() => {});
		try {
			await writeAs(frozen, written);

			const held = JSON.parse(await storedSecretOf(frozen.memberID));
			expect(held.IMAPPassword).toBe(written.imapPassword);
			expect(held.SealedIMAPPassword).toBeNull();
			expect(said).toHaveBeenCalledWith('mail_account.kept_unsealed', expect.objectContaining({ companyID: frozen.companyID }));
		} finally {
			said.mockRestore();
		}
	});
});

describe('resealing what was kept before sealing', () => {
	test('a readable password of a company with a box is sealed, and one without a box is left and named', async () => {
		const readable = { ...(await readAsHost(boxed)), IMAPPassword: 'old-imap', SMTPPassword: 'old-smtp' };
		await keepMemberCredential(client, boxed.memberID, {
			kind: mailAccountCredentialKind,
			externalID: boxed.memberID,
			secret: JSON.stringify({ ...readable, SealedIMAPPassword: null, SealedSMTPPassword: null, MemberID: undefined })
		});

		const report = await resealMailAccounts(client);

		const held = JSON.parse(await storedSecretOf(boxed.memberID));
		expect(held.IMAPPassword).toBe('');
		expect(
			await openedBy(
				boxSecretKey,
				sealedOf(held.SealedIMAPPassword),
				mailPasswordPurpose({ companyID: boxed.companyID, memberID: boxed.memberID }, 'IMAPPassword')
			)
		).toBe('old-imap');
		expect(report.sealed).toContainEqual({ companyID: boxed.companyID, memberID: boxed.memberID });
		expect(report.withoutABox).toContainEqual({ companyID: frozen.companyID, memberID: frozen.memberID });
		expect(JSON.parse(await storedSecretOf(frozen.memberID)).IMAPPassword).toBe(written.imapPassword);
	});
});
