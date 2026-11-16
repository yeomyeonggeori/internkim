import { untrack } from 'svelte';
import { MediaQuery } from 'svelte/reactivity';

const rowKeyAttribute = 'data-reorder-key';
const slideTiming: KeyframeAnimationOptions = { duration: 240, easing: 'cubic-bezier(0.2, 0, 0, 1)' };

export function reorderRowProps(key: string): Record<string, string> {
	return { [rowKeyAttribute]: key };
}

export function slideRowsOnReorder(list: () => HTMLElement | null, order: () => readonly string[]): void {
	const prefersReducedMotion = new MediaQuery('prefers-reduced-motion: reduce');
	let positionsBefore = new Map<string, number>();

	$effect.pre(() => {
		order();
		const element = untrack(list);
		if (element) positionsBefore = rowPositionsOf(element);
	});

	$effect(() => {
		order();
		const element = untrack(list);
		if (!element || untrack(() => prefersReducedMotion.current)) return;
		for (const [row, top] of rowPositionsOf(element)) {
			const before = positionsBefore.get(row);
			const offset = before === undefined ? 0 : before - top;
			if (offset !== 0) rowOf(element, row)?.animate([{ transform: `translateY(${offset}px)` }, { transform: 'none' }], slideTiming);
		}
	});
}

function rowPositionsOf(list: HTMLElement): Map<string, number> {
	const rows = list.querySelectorAll<HTMLElement>(`:scope > [${rowKeyAttribute}]`);
	return new Map([...rows].map((row) => [row.getAttribute(rowKeyAttribute) ?? '', row.offsetTop]));
}

function rowOf(list: HTMLElement, key: string): HTMLElement | null {
	return list.querySelector<HTMLElement>(`:scope > [${rowKeyAttribute}="${CSS.escape(key)}"]`);
}
