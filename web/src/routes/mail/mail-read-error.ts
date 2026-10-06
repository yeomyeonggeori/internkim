import { ToolRefused } from '$lib/tool-answer';
import type { MailPageControllerState } from './mail-page-controller-types';

export class MailReadError extends Error {
	constructor(message: string, readonly status: number) {
		super(message);
		this.name = 'MailReadError';
	}
}

export function isMailAccessDenied(error: unknown): boolean {
	return (error instanceof MailReadError || error instanceof ToolRefused) && (error.status === 401 || error.status === 403);
}

export function discardDeniedMail(controller: MailPageControllerState): void {
	controller.messageListRequestID += 1;
	controller.messageDetailRequestID += 1;
	controller.messages = [];
	controller.mailboxes = [];
	controller.messageListCache.clear();
	controller.messageDetailCache.clear();
	controller.selectedMessage = null;
	controller.requestedMessage = null;
	controller.nextCursor = '';
	controller.hasMoreMessages = false;
	controller.isLoadingMessage = false;
	controller.isLoadingMessages = false;
}
