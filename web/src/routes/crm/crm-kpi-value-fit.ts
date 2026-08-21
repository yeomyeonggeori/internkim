const donutValueSizes = [
	['text-xl', 'leading-6'],
	['text-base', 'leading-5'],
	['text-sm', 'leading-4'],
	['text-xs', 'leading-4']
];

const everySizeClass = donutValueSizes.flat();

export function fitDonutValue(node: HTMLElement, displayValue: string) {
	function overflows(): boolean {
		const measured = node.firstElementChild ?? node;
		return measured.scrollWidth > node.clientWidth;
	}

	function applyLargestSizeThatFits(): void {
		for (const size of donutValueSizes) {
			node.classList.remove(...everySizeClass);
			node.classList.add(...size);
			if (!overflows()) return;
		}
	}

	applyLargestSizeThatFits();
	const observer = new ResizeObserver(applyLargestSizeThatFits);
	observer.observe(node);

	return {
		update(nextValue: string): void {
			if (nextValue === displayValue) return;
			displayValue = nextValue;
			applyLargestSizeThatFits();
		},
		destroy(): void {
			observer.disconnect();
		}
	};
}
