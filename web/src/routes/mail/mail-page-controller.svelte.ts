import {
	loadMailBootstrap,
	openMailSettings,
	saveMailAccountDraft,
	testMailAccountDraft
} from './mail-page-account-actions';
import { openMailCompose, openMailForward, openMailReply, sendMailComposeDraft } from './mail-page-compose-actions';
import {
	hasCachedNextMessagePage,
	loadMessagesPage,
	loadPageMailboxes,
	moveSelectedMailMessage,
	selectMailPageMailbox,
	selectMailPageMessage,
	setMailPageUnreadOnly
} from './mail-page-message-actions';
import { emptyComposeDraft, emptyMailAccount } from './mail-account-draft';
import { resolveMailActorEmail } from './mail-request-actor';
import {
	defaultMailboxes,
	displayedMailboxes,
	mailApiErrorMessages,
	mailMessageBody,
	mailMessageBodyHTML,
	selectedMailboxCountText,
	selectedMailboxLabel,
	visibleMailMessages
} from './mail-page-utils';
import type { ComposeDraft, MailAccount, MailAccountDraft, Mailbox, MailMessage } from './mail-types';
import type { MailComposeFocusField, MailMessagePageCacheEntry, MailPageText } from './mail-page-controller-types';

export function createMailPageController(text: MailPageText) {
	return new MailPageController(text);
}

class MailPageController {
	account = $state<MailAccount>(emptyMailAccount);
	accountDraft = $state<MailAccountDraft>({ ...emptyMailAccount, imapPassword: '', smtpPassword: '' });
	composeDraft = $state<ComposeDraft>(emptyComposeDraft);
	mailboxes = $state<Mailbox[]>([]);
	messages = $state<MailMessage[]>([]);
	messageListCache = new Map<string, MailMessagePageCacheEntry>();
	messageDetailCache = new Map<string, MailMessage>();
	selectedMailbox = $state('INBOX');
	selectedMessage = $state<MailMessage | null>(null);
	searchText = $state('');
	activeSearchText = $state('');
	messagePageIndex = $state(0);
	nextCursor = $state('');
	hasMoreMessages = $state(false);
	isUnreadOnly = $state(false);
	canSelectFirstMessage = $state(true);
	hasLoadedAccount = $state(false);
	isLoading = $state(false);
	isSyncing = $state(false);
	isLoadingMailboxes = $state(false);
	isLoadingMessages = $state(false);
	isLoadingMessage = $state(false);
	isSavingAccount = $state(false);
	isTestingAccount = $state(false);
	isSending = $state(false);
	isSettingsOpen = $state(false);
	isComposeOpen = $state(false);
	composeFocusField = $state<MailComposeFocusField>('to');
	errorMessage = $state('');
	settingsMessage = $state('');
	composeMessage = $state('');
	messageListRequestID = 0;
	messageDetailRequestID = 0;

	constructor(private readonly text: MailPageText) {}

	pageMailboxes = () => displayedMailboxes(this.account, this.mailboxes, defaultMailboxes(this.text));
	visibleMessages = () => visibleMailMessages(this.messages, this.isUnreadOnly);
	canLoadMoreMessages = () => this.hasMoreMessages || hasCachedNextMessagePage(this);
	selectedMessageBody = () => mailMessageBody(this.selectedMessage);
	selectedMessageBodyHTML = () => mailMessageBodyHTML(this.selectedMessage);
	selectedMailboxCountText = () => selectedMailboxCountText(this.pageMailboxes(), this.selectedMailbox, this.text);
	selectedMailboxLabel = () => selectedMailboxLabel(this.pageMailboxes(), this.selectedMailbox);

	loadMail = async () => {
		this.isLoading = !this.hasLoadedAccount && this.messages.length === 0;
		this.isSyncing = true;
		this.errorMessage = '';
		try {
			await loadMailBootstrap(this, this.text);
			this.hasLoadedAccount = true;
			if (!this.account.isConfigured) {
				this.mailboxes = [];
				this.resetMessageList();
				return;
			}
			this.isLoading = false;
			await Promise.all([loadPageMailboxes(this, this.text), this.loadMessages()]);
		} catch (error) {
			this.hasLoadedAccount = true;
			this.errorMessage = error instanceof Error ? error.message : this.text.errors.loadMail;
		} finally {
			this.isLoading = false;
			this.isSyncing = false;
		}
	};

	loadMessages = () => loadMessagesPage(this, this.text, false);

	openMailboxMessage = async (mailbox: string, uid: number) => {
		this.selectedMailbox = mailbox;
		await loadMessagesPage(this, this.text, { mode: 'cache-first', pageIndex: 0 });
		const message = this.messages.find((candidate) => candidate.mailbox === mailbox && candidate.uid === uid);
		if (!message) return;
		this.selectMessage(message);
	};

	loadMoreMessages = () => loadMessagesPage(this, this.text, { mode: 'cache-first', pageIndex: this.messagePageIndex + 1 });

	setUnreadOnly = (isUnreadOnly: boolean) => setMailPageUnreadOnly(this, this.text, isUnreadOnly);

	selectMailbox = (mailboxName: string) => selectMailPageMailbox(this, this.text, mailboxName);

	selectMessage = (message: MailMessage) => selectMailPageMessage(this, this.text, message);

	clearSelectedMessage = () => {
		this.selectedMessage = null;
	};

	openSettings = () => openMailSettings(this);

	openCompose = () => openMailCompose(this);

	openReply = () => openMailReply(this);

	openForward = () => openMailForward(this, this.text);

	saveAccount = () => saveMailAccountDraft(this, this.text);

	testAccount = () => testMailAccountDraft(this, this.text);

	sendMessage = () => sendMailComposeDraft(this, this.text);

	moveSelectedMessage = (targetHint: string) => moveSelectedMailMessage(this, this.text, targetHint);

	mailActorEmail() {
		return resolveMailActorEmail(this.account.email, this.accountDraft.email);
	}

	mailErrors(fallback: string) {
		return mailApiErrorMessages(fallback, this.text.errors.serviceUnavailable);
	}

	resetMessageList() {
		this.messages = [];
		this.messageListCache.clear();
		this.messageDetailCache.clear();
		this.selectedMessage = null;
		this.activeSearchText = '';
		this.messagePageIndex = 0;
		this.nextCursor = '';
		this.hasMoreMessages = false;
	}
}
