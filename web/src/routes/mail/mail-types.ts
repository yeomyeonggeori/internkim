export type MailAccount = {
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

export type MailAccountDraft = MailAccount & {
	imapPassword: string;
	smtpPassword: string;
};

export type MailAccountWritePayload = {
	email: string;
	fromAddress: string;
	displayName: string;
	imapHost: string;
	imapPort: number;
	imapSecurity: string;
	imapUsername: string;
	imapPassword: string;
	smtpHost: string;
	smtpPort: number;
	smtpSecurity: string;
	smtpUsername: string;
	smtpPassword: string;
	defaultMailbox: string;
	sentMailbox: string;
};

export type Mailbox = {
	name: string;
	displayName: string;
	unseen: number;
	total: number;
};

export type MailBootstrap = {
	account: Partial<MailAccount>;
	mailboxes: Mailbox[];
	messages: MailMessage[];
	nextCursor: string;
	hasCachedMailboxes: boolean;
	hasCachedMessages: boolean;
};

export type MailMessage = {
	uid: number;
	mailbox: string;
	subject: string;
	from: string;
	to?: string;
	cc?: string;
	date: string;
	preview: string;
	body?: string;
	bodyHTML?: string;
	isRead: boolean;
};

export type ComposeDraft = {
	to: string;
	cc: string;
	bcc: string;
	subject: string;
	body: string;
};

export type ComposePayload = {
	to: string[];
	cc: string[];
	bcc: string[];
	subject: string;
	body: string;
};
