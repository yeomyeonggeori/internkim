<script lang="ts">
	import { generatePalette, renderGradient, toSeed, type Pattern } from '$lib/avatar-gradient/engine';
	import { cn } from '$lib/utils';

	const minimumDetailSize = 96;

	let {
		seed,
		pattern = 'mesh',
		class: className
	}: {
		seed: string;
		pattern?: Pattern;
		class?: string;
	} = $props();

	let canvas = $state<HTMLCanvasElement | null>(null);
	let isPainted = $state(false);

	const numericSeed = $derived(toSeed(seed));
	const fallbackGradient = $derived(
		`linear-gradient(135deg, ${generatePalette(numericSeed).colors.join(', ')})`
	);

	function paint(element: HTMLCanvasElement, currentSeed: number, currentPattern: Pattern): void {
		const displaySize = element.getBoundingClientRect().width;
		if (displaySize === 0) return;
		const rendered = Math.round(displaySize * (window.devicePixelRatio || 1));
		element.width = rendered;
		element.height = rendered;
		renderGradient(element, currentSeed, {
			pattern: currentPattern,
			displaySize: Math.max(rendered, minimumDetailSize)
		});
		isPainted = true;
	}

	$effect(() => {
		const element = canvas;
		if (!element) return;
		const currentSeed = numericSeed;
		const currentPattern = pattern;
		const repaint = () => paint(element, currentSeed, currentPattern);
		repaint();
		const observer = new ResizeObserver(repaint);
		observer.observe(element);
		return () => observer.disconnect();
	});
</script>

<canvas
	bind:this={canvas}
	class={cn(className, 'absolute -inset-px h-[calc(100%+2px)] w-[calc(100%+2px)]')}
	style={isPainted ? undefined : `background-image: ${fallbackGradient}`}
	aria-hidden="true"
></canvas>
