import type { ChannelMessage } from './channel-api';
import { messageTextBeside } from './message-text-beside';

const pendingMessagePrefix = 'pending-';

export type EditOutcome = 'saved' | 'unchanged' | 'failed';

export function editableTextOf(message: ChannelMessage): string {
	return messageTextBeside(
		message.text,
		(message.attachments ?? []).map((attachment) => attachment.url)
	);
}

export function canEditMessage(message: ChannelMessage, isMine: boolean): boolean {
	if (!isMine || message.isError || message.interaction) return false;
	if (message.id.startsWith(pendingMessagePrefix)) return false;
	return editableTextOf(message) !== '';
}

export async function saveEditing(
	messageID: string,
	originalText: string,
	editedText: string,
	persist: (messageID: string, text: string) => Promise<boolean>
): Promise<EditOutcome> {
	const trimmed = editedText.trim();
	if (trimmed === '' || trimmed === originalText.trim()) return 'unchanged';
	return (await persist(messageID, trimmed)) ? 'saved' : 'failed';
}
