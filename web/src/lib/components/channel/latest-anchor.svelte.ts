import { latestSentAt, type ReadableMessage } from '$lib/messenger/conversation-read-marker';
import { unseenMessageCount } from './channel-unseen-messages';

const awayFromLatestPixels = 200;

export type LatestAnchor = ReturnType<typeof createLatestAnchor>;

export function createLatestAnchor<Message extends ReadableMessage>(options: {
	settledMessages: () => Message[];
	isMine: (message: Message) => boolean;
}) {
	let isAwayFromLatest = $state(false);
	let seenThroughSentAt = $state('');
	const unseenCount = $derived(
		isAwayFromLatest ? unseenMessageCount(options.settledMessages(), seenThroughSentAt, options.isMine) : 0
	);

	$effect(() => {
		if (!isAwayFromLatest) seenThroughSentAt = latestSentAt(options.settledMessages());
	});

	return {
		get isAwayFromLatest() {
			return isAwayFromLatest;
		},
		get unseenCount() {
			return unseenCount;
		},
		follow(scroller: HTMLElement) {
			isAwayFromLatest = Math.abs(scroller.scrollTop) > awayFromLatestPixels;
		},
		reset() {
			isAwayFromLatest = false;
		}
	};
}
