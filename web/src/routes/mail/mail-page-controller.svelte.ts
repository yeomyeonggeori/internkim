import {
	loadMailAccount,
	openMailSettings,
	saveMailAccountDraft,
	testMailAccountDraft
} from './mail-page-account-actions';
import { openMailCompose, openMailReply, sendMailComposeDraft } from './mail-page-compose-actions';
import {
	handleMailMessageListScroll,
	loadMessagesPage,
	loadPageMailboxes,
	moveSelectedMailMessage,
	selectMailPageMailbox,
	selectMailPageMessage,
	toggleSelectedMailMessageRead
} from './mail-page-message-actions';
import { emptyComposeDraft, emptyMailAccount } from './mail-account-draft';
import { resolveMailActorEmail } from './mail-request-actor';
import {
	defaultMailboxes,
	displayedMailboxes,
	mailApiErrorMessages,
	mailMessageBody,
	mailMessageBodyHTML,
	mailMessageCountText,
	visibleMailMessages
} from './mail-page-utils';
import type { ComposeDraft, MailAccount, MailAccountDraft, Mailbox, MailMessage } from './mail-types';
import type { MailPageText } from './mail-page-controller-types';

export function createMailPageController(text: MailPageText) {
	return new MailPageController(text);
}

class MailPageController {
	account = $state<MailAccount>(emptyMailAccount);
	accountDraft = $state<MailAccountDraft>({ ...emptyMailAccount, imapPassword: '', smtpPassword: '' });
	composeDraft = $state<ComposeDraft>(emptyComposeDraft);
	mailboxes = $state<Mailbox[]>([]);
	messages = $state<MailMessage[]>([]);
	messageDetailCache = new Map<string, MailMessage>();
	selectedMailbox = $state('INBOX');
	selectedMessage = $state<MailMessage | null>(null);
	searchText = $state('');
	activeSearchText = $state('');
	nextCursor = $state('');
	hasMoreMessages = $state(false);
	isUnreadOnly = $state(false);
	hasLoadedAccount = $state(false);
	isLoading = $state(false);
	isLoadingMailboxes = $state(false);
	isLoadingMessages = $state(false);
	isLoadingMessage = $state(false);
	isLoadingMore = $state(false);
	isSavingAccount = $state(false);
	isTestingAccount = $state(false);
	isSending = $state(false);
	isSettingsOpen = $state(false);
	isComposeOpen = $state(false);
	errorMessage = $state('');
	settingsMessage = $state('');
	composeMessage = $state('');
	messageListRequestID = 0;
	messageDetailRequestID = 0;

	constructor(private readonly text: MailPageText) {}

	pageMailboxes = () => displayedMailboxes(this.account, this.mailboxes, defaultMailboxes(this.text));
	visibleMessages = () => visibleMailMessages(this.messages, this.isUnreadOnly);
	selectedMessageBody = () => mailMessageBody(this.selectedMessage);
	selectedMessageBodyHTML = () => mailMessageBodyHTML(this.selectedMessage);
	messageCountText = () => mailMessageCountText(this.visibleMessages(), this.text.messageCountSuffix);

	loadMail = async () => {
		this.isLoading = true;
		this.errorMessage = '';
		try {
			await loadMailAccount(this, this.text);
			this.hasLoadedAccount = true;
			if (!this.account.isConfigured) {
				this.mailboxes = [];
				this.resetMessageList();
				return;
			}
			await loadPageMailboxes(this, this.text);
			await this.loadMessages();
		} catch (error) {
			this.hasLoadedAccount = true;
			this.errorMessage = error instanceof Error ? error.message : this.text.errors.loadMail;
		} finally {
			this.isLoading = false;
		}
	};

	loadMessages = () => loadMessagesPage(this, this.text, false);

	selectMailbox = (mailboxName: string) => selectMailPageMailbox(this, mailboxName);

	selectMessage = (message: MailMessage) => selectMailPageMessage(this, this.text, message);

	openSettings = () => openMailSettings(this);

	openCompose = () => openMailCompose(this);

	openReply = () => openMailReply(this);

	saveAccount = () => saveMailAccountDraft(this, this.text);

	testAccount = () => testMailAccountDraft(this, this.text);

	sendMessage = () => sendMailComposeDraft(this, this.text);

	moveSelectedMessage = (targetHint: string) => moveSelectedMailMessage(this, this.text, targetHint);

	toggleSelectedMessageRead = () => toggleSelectedMailMessageRead(this, this.text);

	handleMessageListScroll = (event: Event) => handleMailMessageListScroll(this, this.text, event);

	mailActorEmail() {
		return resolveMailActorEmail(this.account.email, this.accountDraft.email);
	}

	mailErrors(fallback: string) {
		return mailApiErrorMessages(fallback, this.text.errors.serviceUnavailable);
	}

	resetMessageList() {
		this.messages = [];
		this.messageDetailCache.clear();
		this.selectedMessage = null;
		this.nextCursor = '';
		this.hasMoreMessages = false;
	}
}
