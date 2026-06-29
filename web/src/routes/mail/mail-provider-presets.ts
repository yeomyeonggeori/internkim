export type MailProviderID = 'gmail' | 'naver' | 'daum' | 'hanmail' | 'custom';

export type MailSecurity = 'tls' | 'starttls' | 'none';

export type MailLoginAccountMode = 'email' | 'localPart';

export type MailProviderPreset = {
	id: Exclude<MailProviderID, 'custom'>;
	domain: string;
	imapHost: string;
	imapPort: number;
	imapSecurity: MailSecurity;
	smtpHost: string;
	smtpPort: number;
	smtpSecurity: MailSecurity;
	sentMailbox: string;
	loginAccountMode: MailLoginAccountMode;
};

export type MailAddressDraft = {
	localPart: string;
	providerID: MailProviderID;
	customDomain: string;
};

export const DEFAULT_MAIL_PROVIDER_ID: MailProviderID = 'gmail';

export const MAIL_PROVIDER_PRESETS: Record<Exclude<MailProviderID, 'custom'>, MailProviderPreset> = {
	gmail: {
		id: 'gmail',
		domain: 'gmail.com',
		imapHost: 'imap.gmail.com',
		imapPort: 993,
		imapSecurity: 'tls',
		smtpHost: 'smtp.gmail.com',
		smtpPort: 587,
		smtpSecurity: 'starttls',
		sentMailbox: '',
		loginAccountMode: 'email'
	},
	naver: {
		id: 'naver',
		domain: 'naver.com',
		imapHost: 'imap.naver.com',
		imapPort: 993,
		imapSecurity: 'tls',
		smtpHost: 'smtp.naver.com',
		smtpPort: 587,
		smtpSecurity: 'starttls',
		sentMailbox: 'Sent',
		loginAccountMode: 'email'
	},
	daum: {
		id: 'daum',
		domain: 'daum.net',
		imapHost: 'imap.daum.net',
		imapPort: 993,
		imapSecurity: 'tls',
		smtpHost: 'smtp.daum.net',
		smtpPort: 465,
		smtpSecurity: 'tls',
		sentMailbox: 'Sent',
		loginAccountMode: 'localPart'
	},
	hanmail: {
		id: 'hanmail',
		domain: 'hanmail.net',
		imapHost: 'imap.daum.net',
		imapPort: 993,
		imapSecurity: 'tls',
		smtpHost: 'smtp.daum.net',
		smtpPort: 465,
		smtpSecurity: 'tls',
		sentMailbox: 'Sent',
		loginAccountMode: 'localPart'
	}
};

export function mailProviderPreset(providerID: MailProviderID): MailProviderPreset | null {
	if (providerID === 'custom') return null;
	return MAIL_PROVIDER_PRESETS[providerID];
}

export function mailProviderIDFromDomain(domain: string): MailProviderID {
	const normalizedDomain = normalizeEmailPart(domain).toLowerCase();
	for (const preset of Object.values(MAIL_PROVIDER_PRESETS)) {
		if (preset.domain === normalizedDomain) return preset.id;
	}
	return 'custom';
}

export function mailAddressDraftFromEmail(email: string): MailAddressDraft {
	const [localPart, ...domainParts] = normalizeEmailPart(email).split('@');
	const domain = domainParts.join('@').toLowerCase();
	if (!domain) {
		return {
			localPart,
			providerID: DEFAULT_MAIL_PROVIDER_ID,
			customDomain: ''
		};
	}
	const providerID = mailProviderIDFromDomain(domain);
	return {
		localPart,
		providerID,
		customDomain: providerID === 'custom' ? domain : ''
	};
}

export function composeMailAddress(localPart: string, domain: string): string {
	const normalizedLocalPart = normalizeEmailPart(localPart);
	const normalizedDomain = normalizeEmailPart(domain).toLowerCase();
	if (!normalizedLocalPart || !normalizedDomain) return '';
	return `${normalizedLocalPart}@${normalizedDomain}`;
}

export function normalizeEmailPart(value: string): string {
	return value.trim();
}
