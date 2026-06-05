import { sendMailMessage } from './mail-api';
import { composeDraftPayload, emptyComposeDraft } from './mail-account-draft';
import { createMailReplyDraft } from './mail-page-utils';
import type { MailPageControllerState, MailPageText } from './mail-page-controller-types';

export function openMailCompose(controller: MailPageControllerState) {
	controller.composeDraft = { ...emptyComposeDraft };
	controller.composeMessage = '';
	controller.isComposeOpen = true;
}

export function openMailReply(controller: MailPageControllerState) {
	if (!controller.selectedMessage) return;
	controller.composeDraft = createMailReplyDraft(controller.selectedMessage);
	controller.composeMessage = '';
	controller.isComposeOpen = true;
}

export async function sendMailComposeDraft(controller: MailPageControllerState, text: MailPageText) {
	controller.isSending = true;
	controller.composeMessage = '';
	try {
		await sendMailMessage(controller.mailActorEmail(), composeDraftPayload(controller.composeDraft), controller.mailErrors(text.errors.sendMessage));
		controller.isComposeOpen = false;
		await controller.loadMail();
	} catch (error) {
		controller.composeMessage = error instanceof Error ? error.message : text.errors.sendMessage;
	} finally {
		controller.isSending = false;
	}
}
