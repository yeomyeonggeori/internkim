import type { ChannelMessage } from './channel-api';

export type DeleteWarningText = {
	deleteMessageDescription: string;
	deleteThreadDescription: string;
};

export function deleteWarningFor(message: ChannelMessage, text: DeleteWarningText): string {
	const replies = message.thread?.replyCount ?? 0;
	return replies > 0 ? text.deleteThreadDescription : text.deleteMessageDescription;
}
