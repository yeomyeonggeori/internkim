import { fetchMailAccount, fetchMailBootstrap, saveMailAccount, testMailAccount } from './mail-api';
import { createMailAccountDraft, emptyMailAccount, isPasswordNeededAgain, mailAccountDraftPayload } from './mail-account-draft';
import { selectedVisibleMessage } from './mail-message-detail-cache';
import { mailMessagePageSize } from './mail-message-page-cache';
import type { MailPageControllerState, MailPageText } from './mail-page-controller-types';

export async function loadMailBootstrap(controller: MailPageControllerState, text: MailPageText) {
	const searchText = controller.searchText.trim();
	const query = new URLSearchParams({ mailbox: controller.selectedMailbox || 'INBOX', limit: String(mailMessagePageSize) });
	if (searchText) query.set('query', searchText);
	const bootstrap = await fetchMailBootstrap(query, controller.mailErrors(text.errors.connectAccount));
	controller.account = { ...emptyMailAccount, ...bootstrap.account };
	controller.selectedMailbox = controller.selectedMailbox || controller.account.defaultMailbox || 'INBOX';
	controller.accountDraft = createMailAccountDraft(controller.account);
	if (bootstrap.hasCachedMailboxes) controller.mailboxes = bootstrap.mailboxes;
	if (bootstrap.hasCachedMessages) {
		controller.activeSearchText = searchText;
		controller.messagePageIndex = 0;
		controller.messages = bootstrap.messages;
		controller.nextCursor = bootstrap.nextCursor;
		controller.hasMoreMessages = bootstrap.nextCursor !== '';
		controller.selectedMessage = selectedVisibleMessage(controller.visibleMessages(), controller.selectedMessage, controller.canSelectFirstMessage);
	}
	if (!controller.account.isConfigured) {
		controller.mailboxes = [];
		controller.resetMessageList();
	}
}

export async function loadMailAccount(controller: MailPageControllerState, text: MailPageText) {
	controller.account = { ...emptyMailAccount, ...(await fetchMailAccount(controller.mailErrors(text.errors.connectAccount))) };
	controller.selectedMailbox = controller.selectedMailbox || controller.account.defaultMailbox || 'INBOX';
	controller.accountDraft = createMailAccountDraft(controller.account);
}

export function openMailSettings(controller: MailPageControllerState) {
	controller.accountDraft = createMailAccountDraft(controller.account);
	controller.settingsMessage = '';
	controller.isSettingsOpen = true;
}

export async function saveMailAccountDraft(controller: MailPageControllerState, text: MailPageText) {
	if (isPasswordNeededAgain(controller.account, controller.accountDraft)) {
		controller.settingsMessage = text.settingsSheet.passwordNeededAgain;
		return;
	}
	controller.isSavingAccount = true;
	controller.settingsMessage = '';
	try {
		const payload = mailAccountDraftPayload(controller.accountDraft);
		controller.account = { ...emptyMailAccount, ...(await saveMailAccount(payload, controller.mailErrors(text.errors.saveAccount))) };
		controller.accountDraft = createMailAccountDraft(controller.account);
		controller.settingsMessage = text.settingsSheet.saved;
		controller.mailboxes = [];
		controller.selectedMailbox = controller.account.defaultMailbox || 'INBOX';
		controller.resetMessageList();
		await controller.loadMail();
		controller.isSettingsOpen = false;
	} catch (error) {
		controller.settingsMessage = error instanceof Error ? error.message : text.settingsSheet.saveFailed;
	} finally {
		controller.isSavingAccount = false;
	}
}

export async function testMailAccountDraft(controller: MailPageControllerState, text: MailPageText) {
	controller.isTestingAccount = true;
	controller.settingsMessage = '';
	try {
		await testMailAccount(mailAccountDraftPayload(controller.accountDraft), controller.mailErrors(text.settingsSheet.testFailed));
		controller.settingsMessage = text.settingsSheet.testSucceeded;
	} catch (error) {
		controller.settingsMessage = error instanceof Error ? error.message : text.settingsSheet.testFailed;
	} finally {
		controller.isTestingAccount = false;
	}
}
