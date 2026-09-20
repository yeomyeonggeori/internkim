import { createDevAdminMockState, type DevAdminMockState } from './dev-admin-mock';
import type { MailAccount, MailMessage, Mailbox } from './src/routes/mail/mail-types';

export type DevMailMockState = DevAdminMockState & {
	account: MailAccount;
	mailboxes: Mailbox[];
	messages: MailMessage[];
};

export function createDevMailMockState(userEmail: string): DevMailMockState {
	const account = createDevMailAccount(userEmail);
	return {
		...createDevAdminMockState(userEmail),
		account,
		mailboxes: createDevMailboxes(),
		messages: createDevMailMessages(account.email)
	};
}

function createDevMailAccount(userEmail: string): MailAccount {
	return {
		email: userEmail,
		fromAddress: userEmail,
		displayName: 'Dev Mail',
		imapHost: 'imap.gmail.com',
		imapPort: 993,
		imapSecurity: 'tls',
		imapUsername: userEmail,
		smtpHost: 'smtp.gmail.com',
		smtpPort: 587,
		smtpSecurity: 'starttls',
		smtpUsername: userEmail,
		defaultMailbox: 'INBOX',
		sentMailbox: '',
		isConfigured: true,
		hasIMAPPassword: true,
		hasSMTPPassword: true
	};
}

function createDevMailboxes(): Mailbox[] {
	return [
		{ name: 'INBOX', displayName: '받은편지함', unseen: 1, total: 3 },
		{ name: 'Sent', displayName: '보낸메일', unseen: 0, total: 1 },
		{ name: 'Archive', displayName: '보관함', unseen: 0, total: 0 },
		{ name: 'Spam', displayName: '스팸메일함', unseen: 0, total: 0 },
		{ name: 'Trash', displayName: '휴지통', unseen: 0, total: 0 }
	];
}

function createDevMailMessages(userEmail: string): MailMessage[] {
	return [
		{
			uid: 103,
			mailbox: 'INBOX',
			subject: '메일 캐시 동작 확인',
			from: '이샘플 <product@example.com>',
			to: userEmail,
			date: '2026-06-18T10:30:00+09:00',
			preview: '저장된 목록을 먼저 보여주고 뒤에서 새 메일을 확인합니다.',
			body: '저장된 목록을 먼저 보여주고 뒤에서 새 메일을 확인합니다.',
			isRead: false
		},
		{
			uid: 102,
			mailbox: 'INBOX',
			subject: 'Gmail 스타일 부분 로딩',
			from: 'design@example.com',
			to: userEmail,
			date: '2026-06-18T09:00:00+09:00',
			preview: '목록은 유지하고 새로고침 아이콘만 회전합니다.',
			body: '목록은 유지하고 새로고침 아이콘만 회전합니다.',
			isRead: true
		},
		{
			uid: 101,
			mailbox: 'INBOX',
			subject: 'IMAP 연결 테스트',
			from: 'ops@example.com',
			to: userEmail,
			date: '2026-06-17T18:00:00+09:00',
			preview: '개발 mock에서는 연결 테스트가 즉시 성공합니다.',
			body: '개발 mock에서는 연결 테스트가 즉시 성공합니다.',
			isRead: true
		}
	];
}
