import type { SupabaseClient } from '@supabase/supabase-js';
import { keepMemberCredential, memberCredential } from './member-credential';

export const mailCredentialKind = 'mail';

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
	DefaultMailbox: string;
	SentMailbox: string;
};

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
		isConfigured: Boolean(account.IMAPHost && account.SMTPHost && account.IMAPPassword && account.SMTPPassword),
		hasIMAPPassword: account.IMAPPassword !== '',
		hasSMTPPassword: account.SMTPPassword !== ''
	};
}

export function asWritten(
	written: MailAccountAsWritten,
	actorEmail: string,
	held: MailAccount | null
): MailAccount {
	return {
		ActorEmail: actorEmail,
		Email: text(written.email) || held?.Email || '',
		FromAddress: text(written.fromAddress) || text(written.email) || held?.FromAddress || '',
		DisplayName: text(written.displayName),
		IMAPHost: text(written.imapHost),
		IMAPPort: port(written.imapPort, 993),
		IMAPSecurity: text(written.imapSecurity) || 'tls',
		IMAPUsername: text(written.imapUsername),
		IMAPPassword: text(written.imapPassword) || held?.IMAPPassword || '',
		SMTPHost: text(written.smtpHost),
		SMTPPort: port(written.smtpPort, 587),
		SMTPSecurity: text(written.smtpSecurity) || 'starttls',
		SMTPUsername: text(written.smtpUsername),
		SMTPPassword: text(written.smtpPassword) || held?.SMTPPassword || '',
		DefaultMailbox: text(written.defaultMailbox) || 'INBOX',
		SentMailbox: text(written.sentMailbox) || 'Sent'
	};
}

export async function mailAccountOfMember(
	client: SupabaseClient,
	memberID: string
): Promise<MailAccount | null> {
	const credential = await memberCredential(client, memberID, mailCredentialKind);
	if (!credential) return null;
	return readStored(credential.secret);
}

export async function keepMailAccount(
	client: SupabaseClient,
	memberID: string,
	account: MailAccount
): Promise<void> {
	await keepMemberCredential(client, memberID, {
		kind: mailCredentialKind,
		externalID: memberID,
		secret: JSON.stringify(account)
	});
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
		DefaultMailbox: text(held.DefaultMailbox) || 'INBOX',
		SentMailbox: text(held.SentMailbox) || 'Sent'
	};
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
