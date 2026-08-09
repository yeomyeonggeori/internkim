import type { SupabaseClient } from '@supabase/supabase-js';
import { memberCredential } from './member-credential';

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
};

export async function mailAccountOfMember(
	client: SupabaseClient,
	memberID: string
): Promise<MailAccount | null> {
	const credential = await memberCredential(client, memberID, mailCredentialKind);
	if (!credential) return null;

	const stored = readStored(credential.secret);
	if (!stored) return null;
	return stored;
}

function readStored(secret: string): MailAccount | null {
	const parsed = parseJSON(secret);
	if (typeof parsed !== 'object' || parsed === null) return null;

	const held = parsed as Partial<MailAccount>;
	if (!held.IMAPHost || !held.IMAPUsername || !held.SMTPHost || !held.SMTPUsername) return null;
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
		SMTPPassword: text(held.SMTPPassword)
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
