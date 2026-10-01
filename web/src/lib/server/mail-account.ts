import type { SupabaseClient } from '@supabase/supabase-js';
import { sealedSecretSchema, type SealedSecret } from '$lib/company/box';
import { additionalDataOf, sealToBox, type SealPurpose } from '$lib/company/seal-to-box';
import { connectedBoxOf } from './box';
import { keepMemberCredential, memberCredential } from './member-credential';
import { mailAccountCredentialKind } from './public-api/catalog/credential';

export const mailAccountSealInformation = 'internkim mail account';

export type MailPasswordField = 'IMAPPassword' | 'SMTPPassword';

export type MailProtocol = 'IMAP' | 'SMTP';

export type MailAccountOwner = { companyID: string; memberID: string };

export type MailConnection = { host: string; port: number; security: string; username: string };

export type MailAccount = {
	ActorEmail: string;
	Email: string;
	FromAddress: string;
	DisplayName: string;
	IMAPHost: string;
	IMAPPort: number;
	IMAPSecurity: string;
	IMAPUsername: string;
	IMAPPassword: string;
	SMTPHost: string;
	SMTPPort: number;
	SMTPSecurity: string;
	SMTPUsername: string;
	SMTPPassword: string;
	SealedIMAPPassword: SealedSecret | null;
	SealedSMTPPassword: SealedSecret | null;
	DefaultMailbox: string;
	SentMailbox: string;
};

type SealPassword = (password: string, field: MailPasswordField, connection: MailConnection) => Promise<SealedSecret>;

export type MailAccountAsShown = {
	email: string;
	fromAddress: string;
	displayName: string;
	imapHost: string;
	imapPort: number;
	imapSecurity: string;
	imapUsername: string;
	smtpHost: string;
	smtpPort: number;
	smtpSecurity: string;
	smtpUsername: string;
	defaultMailbox: string;
	sentMailbox: string;
	isConfigured: boolean;
	hasIMAPPassword: boolean;
	hasSMTPPassword: boolean;
};

export type MailAccountAsWritten = {
	email?: unknown;
	fromAddress?: unknown;
	displayName?: unknown;
	imapHost?: unknown;
	imapPort?: unknown;
	imapSecurity?: unknown;
	imapUsername?: unknown;
	imapPassword?: unknown;
	smtpHost?: unknown;
	smtpPort?: unknown;
	smtpSecurity?: unknown;
	smtpUsername?: unknown;
	smtpPassword?: unknown;
	defaultMailbox?: unknown;
	sentMailbox?: unknown;
};

export function asShown(account: MailAccount | null): MailAccountAsShown | null {
	if (!account) return null;
	return {
		email: account.Email,
		fromAddress: account.FromAddress,
		displayName: account.DisplayName,
		imapHost: account.IMAPHost,
		imapPort: account.IMAPPort,
		imapSecurity: account.IMAPSecurity,
		imapUsername: account.IMAPUsername,
		smtpHost: account.SMTPHost,
		smtpPort: account.SMTPPort,
		smtpSecurity: account.SMTPSecurity,
		smtpUsername: account.SMTPUsername,
		defaultMailbox: account.DefaultMailbox,
		sentMailbox: account.SentMailbox,
		isConfigured: Boolean(
			account.IMAPHost && account.SMTPHost && hasIMAPPassword(account) && hasSMTPPassword(account)
		),
		hasIMAPPassword: hasIMAPPassword(account),
		hasSMTPPassword: hasSMTPPassword(account)
	};
}

function hasIMAPPassword(account: MailAccount): boolean {
	return account.IMAPPassword !== '' || account.SealedIMAPPassword !== null;
}

function hasSMTPPassword(account: MailAccount): boolean {
	return account.SMTPPassword !== '' || account.SealedSMTPPassword !== null;
}

export function asWritten(
	written: MailAccountAsWritten,
	actorEmail: string,
	held: MailAccount | null
): MailAccount {
	const account: MailAccount = {
		ActorEmail: actorEmail,
		Email: text(written.email) || held?.Email || '',
		FromAddress: text(written.fromAddress) || text(written.email) || held?.FromAddress || '',
		DisplayName: text(written.displayName),
		IMAPHost: text(written.imapHost),
		IMAPPort: port(written.imapPort, 993),
		IMAPSecurity: text(written.imapSecurity) || 'tls',
		IMAPUsername: text(written.imapUsername),
		IMAPPassword: text(written.imapPassword),
		SMTPHost: text(written.smtpHost),
		SMTPPort: port(written.smtpPort, 587),
		SMTPSecurity: text(written.smtpSecurity) || 'starttls',
		SMTPUsername: text(written.smtpUsername),
		SMTPPassword: text(written.smtpPassword),
		SealedIMAPPassword: null,
		SealedSMTPPassword: null,
		DefaultMailbox: text(written.defaultMailbox) || 'INBOX',
		SentMailbox: text(written.sentMailbox) || 'Sent'
	};
	return held ? withHeldPasswords(account, held) : account;
}

export function imapConnectionOf(account: MailAccount): MailConnection {
	return { host: account.IMAPHost, port: account.IMAPPort, security: account.IMAPSecurity, username: account.IMAPUsername };
}

export function smtpConnectionOf(account: MailAccount): MailConnection {
	return { host: account.SMTPHost, port: account.SMTPPort, security: account.SMTPSecurity, username: account.SMTPUsername };
}

function isSameConnection(first: MailConnection, second: MailConnection): boolean {
	return (
		first.host === second.host &&
		first.port === second.port &&
		first.security === second.security &&
		first.username === second.username
	);
}

function withHeldPasswords(account: MailAccount, held: MailAccount): MailAccount {
	const keepsIMAP = !account.IMAPPassword && isSameConnection(imapConnectionOf(account), imapConnectionOf(held));
	const keepsSMTP = !account.SMTPPassword && isSameConnection(smtpConnectionOf(account), smtpConnectionOf(held));
	return {
		...account,
		IMAPPassword: keepsIMAP ? held.IMAPPassword : account.IMAPPassword,
		SealedIMAPPassword: keepsIMAP ? held.SealedIMAPPassword : null,
		SMTPPassword: keepsSMTP ? held.SMTPPassword : account.SMTPPassword,
		SealedSMTPPassword: keepsSMTP ? held.SealedSMTPPassword : null
	};
}

export function passwordsLostToAServerChange(account: MailAccount, held: MailAccount | null): MailProtocol[] {
	const lost: MailProtocol[] = [];
	if (!held) return lost;
	if (hasIMAPPassword(held) && !hasIMAPPassword(account)) lost.push('IMAP');
	if (hasSMTPPassword(held) && !hasSMTPPassword(account)) lost.push('SMTP');
	return lost;
}

export async function mailAccountOfMember(
	client: SupabaseClient,
	memberID: string
): Promise<MailAccount | null> {
	const credential = await memberCredential(client, memberID, mailAccountCredentialKind);
	if (!credential) return null;
	return readStored(credential.secret);
}

export async function keepMailAccount(
	client: SupabaseClient,
	owner: MailAccountOwner,
	account: MailAccount
): Promise<MailAccount> {
	const box = await connectedBoxOf(client, owner.companyID);
	const kept = box
		? await sealedPasswords(account, passwordSealer(box.encryptionKey, owner))
		: unsealedOnTheFrozenDevicePath(owner, account);
	await keepMemberCredential(client, owner.memberID, {
		kind: mailAccountCredentialKind,
		externalID: owner.memberID,
		secret: JSON.stringify(kept)
	});
	return kept;
}

function unsealedOnTheFrozenDevicePath(owner: MailAccountOwner, account: MailAccount): MailAccount {
	console.error('mail_account.kept_unsealed', {
		companyID: owner.companyID,
		memberID: owner.memberID,
		reason: 'the company has no box encryption key, so its mail password stays readable in Vault'
	});
	return account;
}

export function mailPasswordPurpose(
	owner: MailAccountOwner,
	field: MailPasswordField,
	connection: MailConnection
): SealPurpose {
	return {
		information: mailAccountSealInformation,
		additionalData: additionalDataOf(
			owner.companyID,
			owner.memberID,
			'mail',
			field,
			connection.host,
			String(connection.port),
			connection.security,
			connection.username
		)
	};
}

export function passwordSealer(boxEncryptionKey: string, owner: MailAccountOwner): SealPassword {
	return (password, field, connection) =>
		sealToBox(password, boxEncryptionKey, mailPasswordPurpose(owner, field, connection));
}

export async function sealedPasswords(account: MailAccount, seal: SealPassword): Promise<MailAccount> {
	return {
		...account,
		IMAPPassword: '',
		SMTPPassword: '',
		SealedIMAPPassword: account.IMAPPassword
			? await seal(account.IMAPPassword, 'IMAPPassword', imapConnectionOf(account))
			: account.SealedIMAPPassword,
		SealedSMTPPassword: account.SMTPPassword
			? await seal(account.SMTPPassword, 'SMTPPassword', smtpConnectionOf(account))
			: account.SealedSMTPPassword
	};
}

function readStored(secret: string): MailAccount | null {
	const parsed = parseJSON(secret);
	if (typeof parsed !== 'object' || parsed === null) return null;

	const held = parsed as Partial<MailAccount>;
	if (!text(held.IMAPHost) || !text(held.SMTPHost)) return null;
	return {
		ActorEmail: text(held.ActorEmail),
		Email: text(held.Email),
		FromAddress: text(held.FromAddress),
		DisplayName: text(held.DisplayName),
		IMAPHost: text(held.IMAPHost),
		IMAPPort: port(held.IMAPPort, 993),
		IMAPSecurity: text(held.IMAPSecurity) || 'tls',
		IMAPUsername: text(held.IMAPUsername),
		IMAPPassword: text(held.IMAPPassword),
		SMTPHost: text(held.SMTPHost),
		SMTPPort: port(held.SMTPPort, 587),
		SMTPSecurity: text(held.SMTPSecurity) || 'starttls',
		SMTPUsername: text(held.SMTPUsername),
		SMTPPassword: text(held.SMTPPassword),
		SealedIMAPPassword: sealedSecretOf(held.SealedIMAPPassword),
		SealedSMTPPassword: sealedSecretOf(held.SealedSMTPPassword),
		DefaultMailbox: text(held.DefaultMailbox) || 'INBOX',
		SentMailbox: text(held.SentMailbox) || 'Sent'
	};
}

function sealedSecretOf(offered: unknown): SealedSecret | null {
	const parsed = sealedSecretSchema.safeParse(offered);
	return parsed.success ? parsed.data : null;
}

function parseJSON(secret: string): unknown {
	try {
		return JSON.parse(secret);
	} catch {
		return null;
	}
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}

function port(offered: unknown, fallback: number): number {
	return typeof offered === 'number' && offered > 0 && offered < 65536 ? offered : fallback;
}
