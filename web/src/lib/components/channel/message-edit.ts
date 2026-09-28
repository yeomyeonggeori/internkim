import type { ChannelMessage } from './channel-api';
import { messageTextBeside } from './message-text-beside';

const pendingMessagePrefix = 'pending-';

export type EditingMessage = { messageID: string; originalText: string; draftBeforeEditing: string };

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

export function startEditing(
	message: ChannelMessage,
	currentDraft: string,
	alreadyEditing: EditingMessage | null
): EditingMessage {
	return {
		messageID: message.id,
		originalText: editableTextOf(message),
		draftBeforeEditing: alreadyEditing ? alreadyEditing.draftBeforeEditing : currentDraft
	};
}

export async function saveEditing(
	editing: EditingMessage,
	editedText: string,
	persist: (messageID: string, text: string) => Promise<boolean>
): Promise<EditOutcome> {
	const trimmed = editedText.trim();
	if (trimmed === '' || trimmed === editing.originalText.trim()) return 'unchanged';
	return (await persist(editing.messageID, trimmed)) ? 'saved' : 'failed';
}

export type EditableComposer = {
	text: () => string;
	setText: (text: string) => void;
	editing: () => EditingMessage | null;
	setEditing: (editing: EditingMessage | null) => void;
	focus: () => void;
};

export type ComposerEditing = {
	begin: (message: ChannelMessage) => void;
	cancel: () => void;
	save: (persist: (messageID: string, text: string) => Promise<boolean>) => Promise<void>;
};

export function composerEditing(composer: EditableComposer): ComposerEditing {
	function cancel(): void {
		composer.setText(composer.editing()?.draftBeforeEditing ?? '');
		composer.setEditing(null);
	}

	return {
		begin(message) {
			const editing = startEditing(message, composer.text(), composer.editing());
			composer.setEditing(editing);
			composer.setText(editing.originalText);
			composer.focus();
		},
		cancel,
		async save(persist) {
			const editing = composer.editing();
			if (!editing) return;
			if ((await saveEditing(editing, composer.text(), persist)) !== 'failed') cancel();
		}
	};
}
