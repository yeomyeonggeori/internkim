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
	setMailMessageRead,
	setMailPageUnreadOnly
} from './mail-page-message-actions';
import { emptyComposeDraft, emptyMailAccount } from './mail-account-draft';
import { resolveMailActorEmail } from './mail-request-actor';
import {
	availableMailMoveTargets,
	defaultMailboxes,
	displayedMailboxes,
	mailApiErrorMessages,
	mailMessageBody,
	mailMessageBodyHTML,
	selectedMailboxLabel,
	visibleMailMessages
} from './mail-page-utils';
import type { MailMoveTarget } from './mail-page-utils';
import type { ComposeDraft, MailAccount, MailAccountDraft, Mailbox, MailMessage } from './mail-types';
import { discardDeniedMail, isMailAccessDenied } from './mail-read-error';
import type { MailComposeFocusField, MailMessagePageCacheEntry, MailPageText, RequestedMailMessage } from './mail-page-controller-types';

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
	requestedMessage = $state<RequestedMailMessage | null>(null);
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
	moveTargets = () => availableMailMoveTargets(this.pageMailboxes());
	visibleMessages = () => visibleMailMessages(this.messages, this.isUnreadOnly);
	canLoadMoreMessages = () => this.hasMoreMessages || hasCachedNextMessagePage(this);
	selectedMessageBody = () => mailMessageBody(this.selectedMessage);
	selectedMessageBodyHTML = () => mailMessageBodyHTML(this.selectedMessage);
	selectedMailboxLabel = () => selectedMailboxLabel(this.pageMailboxes(), this.selectedMailbox);

	loadMail = async () => {
		const actorEmail = this.mailActorEmail();
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
			await this.resolveRequestedMessage();
		} catch (error) {
			if (isMailAccessDenied(error) && actorEmail === this.mailActorEmail()) discardDeniedMail(this);
			this.hasLoadedAccount = true;
			this.errorMessage = error instanceof Error ? error.message : this.text.errors.loadMail;
		} finally {
			this.isLoading = false;
			this.isSyncing = false;
		}
	};

	loadMessages = () => loadMessagesPage(this, this.text, false);

	searchMessages = () => {
		this.requestedMessage = null;
		return loadMessagesPage(this, this.text, { mode: 'cache-first', pageIndex: 0 });
	};

	openMailboxMessage = async (mailbox: string, uid: number) => {
		this.requestMailboxMessage({ mailbox, uid });
		await loadMessagesPage(this, this.text, { mode: 'cache-first', pageIndex: 0 });
		await this.resolveRequestedMessage();
	};

	requestMailboxMessage = (message: RequestedMailMessage) => {
		this.requestedMessage = message;
		this.selectedMailbox = message.mailbox;
		this.searchText = '';
		this.selectedMessage = null;
	};

	private async resolveRequestedMessage() {
		const requested = this.requestedMessage;
		if (!requested) return;
		const actor = this.mailActorEmail();
		const visitedCursors = new Set<string>();
		while (this.requestedMessage === requested && this.selectedMailbox === requested.mailbox && this.mailActorEmail() === actor) {
			if (this.errorMessage) return;
			const message = this.messages.find((candidate) => candidate.mailbox === requested.mailbox && candidate.uid === requested.uid);
			if (message) {
				this.requestedMessage = null;
				this.selectMessage(message);
				return;
			}
			if (!this.canLoadMoreMessages()) {
				this.errorMessage = this.text.errors.messageNotFound;
				return;
			}
			const pageIndex = this.messagePageIndex;
			if (visitedCursors.has(this.nextCursor)) {
				this.errorMessage = this.text.errors.loadMessages;
				return;
			}
			visitedCursors.add(this.nextCursor);
			await this.loadMoreMessages();
			if (this.messagePageIndex === pageIndex) return;
		}
	}

	loadMoreMessages = () => loadMessagesPage(this, this.text, { mode: 'cache-first', pageIndex: this.messagePageIndex + 1 });

	setUnreadOnly = (isUnreadOnly: boolean) => setMailPageUnreadOnly(this, this.text, isUnreadOnly);

	selectMailbox = (mailboxName: string) => {
		this.requestedMessage = null;
		return selectMailPageMailbox(this, this.text, mailboxName);
	};

	selectMessage = (message: MailMessage) => {
		this.requestedMessage = null;
		selectMailPageMessage(this, this.text, message);
	};

	clearSelectedMessage = () => {
		this.requestedMessage = null;
		this.selectedMessage = null;
	};

	markSelectedMessageUnread = async () => {
		if (!this.selectedMessage) return;
		await setMailMessageRead(this, this.text, this.selectedMessage, false);
		this.canSelectFirstMessage = false;
		this.selectedMessage = null;
	};

	openSettings = () => openMailSettings(this);

	openCompose = () => openMailCompose(this);

	openReply = () => openMailReply(this);

	openForward = () => openMailForward(this, this.text);

	saveAccount = () => saveMailAccountDraft(this, this.text);

	testAccount = () => testMailAccountDraft(this, this.text);

	sendMessage = () => sendMailComposeDraft(this, this.text);

	moveSelectedMessage = (target: MailMoveTarget) => moveSelectedMailMessage(this, this.text, target);

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
