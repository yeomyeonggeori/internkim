import { fetchMailAccount, saveMailAccount, testMailAccount } from './mail-api';
import { createMailAccountDraft, emptyMailAccount, mailAccountDraftPayload } from './mail-account-draft';
import { currentMailHostname, isLocalMailHostname, rememberLocalMailActorEmail, resolveMailAccountSaveActorEmail } from './mail-request-actor';
import type { MailPageControllerState, MailPageText } from './mail-page-controller-types';

export async function loadMailAccount(controller: MailPageControllerState, text: MailPageText) {
	const actorEmail = controller.mailActorEmail();
	if (!actorEmail && isLocalMailHostname(currentMailHostname())) {
		controller.account = emptyMailAccount;
		controller.selectedMailbox = controller.account.defaultMailbox || 'INBOX';
		controller.accountDraft = createMailAccountDraft(controller.account);
		return;
	}
	controller.account = { ...emptyMailAccount, ...(await fetchMailAccount(actorEmail, controller.mailErrors(text.errors.connectAccount))) };
	controller.selectedMailbox = controller.selectedMailbox || controller.account.defaultMailbox || 'INBOX';
	controller.accountDraft = createMailAccountDraft(controller.account);
}

export function openMailSettings(controller: MailPageControllerState) {
	controller.accountDraft = createMailAccountDraft(controller.account);
	controller.settingsMessage = '';
	controller.isSettingsOpen = true;
}

export async function saveMailAccountDraft(controller: MailPageControllerState, text: MailPageText) {
	controller.isSavingAccount = true;
	controller.settingsMessage = '';
	try {
		const actorEmail = resolveMailAccountSaveActorEmail(controller.accountDraft.email);
		const payload = mailAccountDraftPayload(controller.accountDraft);
		controller.account = { ...emptyMailAccount, ...(await saveMailAccount(actorEmail, payload, controller.mailErrors(text.errors.saveAccount))) };
		rememberLocalMailActorEmail(actorEmail);
		controller.accountDraft = createMailAccountDraft(controller.account);
		controller.settingsMessage = text.settingsSheet.saved;
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
		const actorEmail = resolveMailAccountSaveActorEmail(controller.accountDraft.email);
		await testMailAccount(actorEmail, mailAccountDraftPayload(controller.accountDraft), controller.mailErrors(text.settingsSheet.testFailed));
		controller.settingsMessage = text.settingsSheet.testSucceeded;
	} catch (error) {
		controller.settingsMessage = error instanceof Error ? error.message : text.settingsSheet.testFailed;
	} finally {
		controller.isTestingAccount = false;
	}
}
