import { sendMailMessage } from './mail-api';
import { composeDraftPayload, emptyComposeDraft } from './mail-account-draft';
import { createMailForwardDraft, createMailReplyDraft } from './mail-page-utils';
import type { MailPageControllerState, MailPageText } from './mail-page-controller-types';
import type { MailMessage } from './mail-types';

export function openMailCompose(controller: MailPageControllerState) {
	controller.composeDraft = { ...emptyComposeDraft };
	controller.composeMessage = '';
	controller.composeFocusField = 'to';
	controller.isComposeOpen = true;
}

export function openMailReply(controller: MailPageControllerState) {
	if (!controller.selectedMessage) return;
	controller.composeDraft = createMailReplyDraft(controller.selectedMessage);
	controller.composeMessage = '';
	controller.composeFocusField = 'body';
	controller.isComposeOpen = true;
}

export function openMailForward(controller: MailPageControllerState, text: MailPageText) {
	if (!controller.selectedMessage) return;
	controller.composeDraft = createMailForwardDraft(controller.selectedMessage, forwardedHeader(controller.selectedMessage, text));
	controller.composeMessage = '';
	controller.composeFocusField = 'to';
	controller.isComposeOpen = true;
}

function forwardedHeader(message: MailMessage, text: MailPageText) {
	const headerLines = [text.forwardedHeader, `${text.forwardedFrom} ${message.from}`, `${text.fields.subject}: ${message.subject}`];
	if (message.to) headerLines.push(`${text.to}: ${message.to}`);
	return headerLines.join('\n');
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
