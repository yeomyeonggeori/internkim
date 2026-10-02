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

	function paint(element: HTMLCanvasElement, displaySize: number, currentSeed: number, currentPattern: Pattern): void {
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
		isPainted = false;
		let paintedSize = 0;
		const resize = new ResizeObserver((entries) => {
			const displaySize = entries[0]?.contentRect.width ?? 0;
			if (!displaySize || paintedSize === displaySize) return;
			paintedSize = displaySize;
			paint(element, displaySize, currentSeed, currentPattern);
		});
		const visibility = new IntersectionObserver((entries) => {
			if (entries[0]?.isIntersecting) resize.observe(element);
			else resize.unobserve(element);
		});
		visibility.observe(element);
		return () => { visibility.disconnect(); resize.disconnect(); };
	});
</script>

<canvas
	bind:this={canvas}
	class={cn(className, 'absolute -inset-px h-[calc(100%+2px)] w-[calc(100%+2px)]')}
	style={isPainted ? undefined : `background-image: ${fallbackGradient}`}
	aria-hidden="true"
></canvas>
