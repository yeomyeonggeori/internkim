import type { MailErrorMessages } from './mail-api';
import type { ComposeDraft, MailAccount, MailAccountDraft, Mailbox, MailMessage } from './mail-types';
import type { mailText } from './text';

export type MailPageText = (typeof mailText)[keyof typeof mailText];

export type MailComposeFocusField = 'to' | 'body';

export type MailMessagePageCacheEntry = {
	actorEmail: string;
	mailbox: string;
	searchText: string;
	pageIndex: number;
	cursor: string;
	messages: MailMessage[];
	nextCursor: string;
	fetchedAt: number;
};

export type MailPageControllerState = {
	account: MailAccount;
	accountDraft: MailAccountDraft;
	composeDraft: ComposeDraft;
	mailboxes: Mailbox[];
	messages: MailMessage[];
	messageListCache: Map<string, MailMessagePageCacheEntry>;
	messageDetailCache: Map<string, MailMessage>;
	selectedMailbox: string;
	selectedMessage: MailMessage | null;
	searchText: string;
	activeSearchText: string;
	messagePageIndex: number;
	nextCursor: string;
	hasMoreMessages: boolean;
	isUnreadOnly: boolean;
	canSelectFirstMessage: boolean;
	hasLoadedAccount: boolean;
	isLoading: boolean;
	isSyncing: boolean;
	isLoadingMailboxes: boolean;
	isLoadingMessages: boolean;
	isLoadingMessage: boolean;
	isSavingAccount: boolean;
	isTestingAccount: boolean;
	isSending: boolean;
	isSettingsOpen: boolean;
	isComposeOpen: boolean;
	composeFocusField: MailComposeFocusField;
	errorMessage: string;
	settingsMessage: string;
	composeMessage: string;
	messageListRequestID: number;
	messageDetailRequestID: number;
	pageMailboxes: () => Mailbox[];
	visibleMessages: () => MailMessage[];
	canLoadMoreMessages: () => boolean;
	mailActorEmail: () => string;
	mailErrors: (fallback: string) => MailErrorMessages;
	resetMessageList: () => void;
	loadMail: () => Promise<void>;
	loadMessages: () => Promise<void>;
	loadMoreMessages: () => Promise<void>;
	setUnreadOnly: (isUnreadOnly: boolean) => Promise<void>;
};
