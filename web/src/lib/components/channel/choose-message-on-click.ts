export type ChooseMessageOnClickOptions = {
	enabled: boolean;
	onChoose: (messageID: string) => void;
};

const interceptedEvents = ['click', 'contextmenu', 'pointerdown', 'pointerup'] as const;

export function chooseMessageOnClick(
	node: HTMLElement,
	initialOptions: ChooseMessageOnClickOptions
): { update(nextOptions: ChooseMessageOnClickOptions): void; destroy(): void } {
	let options = initialOptions;

	const intercept = (event: Event): void => {
		if (!options.enabled) return;
		event.stopPropagation();
		if (event.type === 'pointerdown' || event.type === 'pointerup') return;
		event.preventDefault();
		if (event.type !== 'click' || !(event.target instanceof Element)) return;
		const row = event.target.closest<HTMLElement>('[data-message-id]');
		const messageID = row?.dataset.messageId;
		if (messageID && node.contains(row)) options.onChoose(messageID);
	};

	for (const eventName of interceptedEvents) node.addEventListener(eventName, intercept, { capture: true });

	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			for (const eventName of interceptedEvents) node.removeEventListener(eventName, intercept, { capture: true });
		}
	};
}
