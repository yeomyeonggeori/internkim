import { toast } from 'svelte-sonner';
import { MessengerRefusal } from '$lib/messenger/messenger-api';
import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
import { copyText } from '$lib/hooks/use-clipboard.svelte';
import { deleteWarningFor } from './channel-delete-warning';
import {
	addChannelReaction,
	deleteChannelMessage,
	editChannelMessage,
	removeChannelReaction,
	type ChannelMessage,
	type ChannelMessageReaction,
	type ChannelParticipant
} from './channel-api';
import { reactionsAfter, reactionValueFor } from './channel-reactions';
import { copyPicture, type MessageCopy } from './message-copy';

export type MessageActionText = {
	messageCopied: string;
	copyFailed: string;
	pictureCopied: string;
	pictureCopyFailed: string;
	deleteMessageTitle: string;
	deleteMessageDescription: string;
	deleteThreadDescription: string;
	deleteMessageFailed: string;
	editMessageFailed: string;
	deleteThreadPartly: string;
	reactionFailed: string;
	reactionRemoveFailed: string;
	delete: string;
	cancel: string;
};

export type MessageActionHost = {
	channelID: () => string | undefined;
	reader: () => ChannelParticipant;
	text: () => MessageActionText;
	showReactions: (messageID: string, reactions: ChannelMessageReaction[]) => void;
	forgetMessage: (messageID: string) => void;
	readAgain: () => Promise<void>;
};

export type MessageActions = {
	reactWith: (message: ChannelMessage, glyph: string) => Promise<void>;
	toggleReaction: (message: ChannelMessage, reaction: ChannelMessageReaction) => Promise<void>;
	copy: (wanted: MessageCopy) => Promise<void>;
	askToDelete: (message: ChannelMessage) => void;
	saveEdit: (messageID: string, text: string) => Promise<boolean>;
};

export function messageActionsFor(host: MessageActionHost): MessageActions {
	async function changeReaction(message: ChannelMessage, value: string, glyph: string, isAdding: boolean): Promise<void> {
		const before = message.reactions ?? [];
		host.showReactions(message.id, reactionsAfter(before, { value, glyph, isAdding, person: host.reader() }));
		try {
			if (isAdding) await addChannelReaction(message.id, value, host.channelID());
			else await removeChannelReaction(message.id, value, host.channelID());
		} catch (failure) {
			console.warn('the messenger did not take the reaction change', failure);
			host.showReactions(message.id, before);
			toast.error(isAdding ? host.text().reactionFailed : host.text().reactionRemoveFailed);
			return;
		}
		await host.readAgain();
	}

	async function deleteMessage(message: ChannelMessage): Promise<void> {
		try {
			await deleteChannelMessage(message.id, host.channelID());
		} catch (failure) {
			if (!(failure instanceof MessengerRefusal) || failure.reason !== 'thread-partly-deleted') {
				console.warn('the messenger did not delete the message', failure);
				toast.error(host.text().deleteMessageFailed);
				return;
			}
			toast.warning(host.text().deleteThreadPartly.replace('{count}', String(failure.remaining ?? '')));
		}
		host.forgetMessage(message.id);
		await host.readAgain();
	}

	return {
		async reactWith(message, glyph) {
			const reactions = message.reactions ?? [];
			const value = reactionValueFor(glyph, reactions);
			const mine = reactions.find((reaction) => reaction.value === value)?.reactedByMe ?? false;
			await changeReaction(message, value, glyph, !mine);
		},
		async toggleReaction(message, reaction) {
			await changeReaction(message, reaction.value, reaction.emoji, !(reaction.reactedByMe ?? false));
		},
		async copy(wanted) {
			if (wanted.kind === 'nothing') return;
			const text = host.text();
			if (wanted.kind === 'picture') {
				const outcome = await copyPicture(wanted.address);
				if (outcome === 'success') toast.success(text.pictureCopied);
				else toast.error(text.pictureCopyFailed);
				return;
			}
			const outcome = await copyText(wanted.text);
			if (outcome === 'success') toast.success(text.messageCopied);
			else toast.error(text.copyFailed);
		},
		async saveEdit(messageID, text) {
			try {
				await editChannelMessage(messageID, text, host.channelID());
			} catch (failure) {
				console.warn('the messenger did not take the edit', failure);
				toast.error(host.text().editMessageFailed);
				return false;
			}
			await host.readAgain();
			return true;
		},
		askToDelete(message) {
			const text = host.text();
			confirmDelete({
				title: text.deleteMessageTitle,
				description: deleteWarningFor(message, text),
				confirm: { text: text.delete },
				cancel: { text: text.cancel },
				onConfirm: () => deleteMessage(message)
			});
		}
	};
}
