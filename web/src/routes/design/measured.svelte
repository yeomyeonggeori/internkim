<script lang="ts">
	import type { Snippet } from 'svelte';

	type Measure = 'font' | 'width' | 'radius';

	let {
		measure,
		class: className = '',
		style = '',
		children
	}: { measure: Measure; class?: string; style?: string; children?: Snippet } = $props();

	let element = $state<HTMLElement | null>(null);
	let reading = $state('');

	function radiusReading(radius: string): string {
		return Number.parseFloat(radius) > 9999 ? '∞' : radius;
	}

	function readingOf(target: HTMLElement): string {
		const computed = getComputedStyle(target);
		if (measure === 'font') return `${computed.fontSize} / ${computed.lineHeight}`;
		if (measure === 'radius') return radiusReading(computed.borderTopLeftRadius);
		return `${Math.round(target.getBoundingClientRect().width)}px`;
	}

	$effect(() => {
		if (!element) return;
		const target = element;
		const observer = new ResizeObserver(() => (reading = readingOf(target)));
		observer.observe(target);
		return () => observer.disconnect();
	});
</script>

<div class="flex min-w-0 items-center gap-3">
	<div bind:this={element} class={className} {style}>{@render children?.()}</div>
	<span class="text-muted-foreground shrink-0 font-mono text-[10px] tabular-nums">{reading}</span>
</div>
