export type OlderHistory = {
	holds: () => boolean;
	hasMore: () => boolean;
	isLoading: () => boolean;
	loadOlder: () => Promise<void>;
};

const mostPagesLoadedToReveal = 20;
const loadingCheckMilliseconds = 100;
const revealFlashMilliseconds = 1600;

export async function loadHistoryUntilHeld(history: OlderHistory): Promise<boolean> {
	for (let page = 0; page < mostPagesLoadedToReveal && !history.holds(); page++) {
		while (history.isLoading()) await new Promise((resolve) => setTimeout(resolve, loadingCheckMilliseconds));
		if (history.holds() || !history.hasMore()) break;
		await history.loadOlder();
	}
	return history.holds();
}

export function revealMessageElement(scroller: HTMLElement, messageID: string): boolean {
	const element = scroller.querySelector<HTMLElement>(`[data-message-id="${CSS.escape(messageID)}"]`);
	if (!element) return false;
	element.scrollIntoView({ block: 'center', behavior: 'smooth' });
	element.animate(
		[{ backgroundColor: 'var(--muted)' }, { backgroundColor: 'transparent' }],
		{ duration: revealFlashMilliseconds, easing: 'ease-out' }
	);
	return true;
}
