<script lang="ts">
	import { onDestroy, onMount } from 'svelte';

	let {
		scroller,
		firstID,
		lastID
	}: {
		scroller: HTMLElement | null;
		firstID: string;
		lastID: string;
	} = $props();

	type Frame = { top: number; left: number; width: number; height: number };

	let host = $state<HTMLDivElement | null>(null);
	let frame = $state<Frame | null>(null);
	let animationFrame = 0;

	function rowNamed(messageID: string): HTMLElement | null {
		if (!scroller || messageID === '') return null;
		return scroller.querySelector<HTMLElement>(`[data-message-id="${CSS.escape(messageID)}"]`);
	}

	function measure(): void {
		const first = rowNamed(firstID);
		const last = rowNamed(lastID);
		if (!host || !scroller || !first || !last) {
			frame = null;
			return;
		}
		const hostBox = host.getBoundingClientRect();
		const scrollerBox = scroller.getBoundingClientRect();
		const top = first.getBoundingClientRect().top - hostBox.top;
		const bottom = last.getBoundingClientRect().bottom - hostBox.top;
		frame = { top, left: scrollerBox.left - hostBox.left, width: scroller.clientWidth, height: bottom - top };
	}

	function follow(): void {
		measure();
		animationFrame = requestAnimationFrame(follow);
	}

	onMount(follow);
	onDestroy(() => cancelAnimationFrame(animationFrame));
</script>

<div bind:this={host} aria-hidden="true" class="pointer-events-none absolute inset-0 z-[5] overflow-hidden" data-capture-overlay>
	{#if frame}
		<div
			class="absolute shadow-[0_0_0_100vmax_rgb(0_0_0/0.55)]"
			style:top="{frame.top}px"
			style:left="{frame.left}px"
			style:width="{frame.width}px"
			style:height="{frame.height}px"
			data-capture-frame
		>
			<span class="absolute -top-0.5 -left-0.5 size-4 border-t-2 border-l-2 border-primary"></span>
			<span class="absolute -top-0.5 -right-0.5 size-4 border-t-2 border-r-2 border-primary"></span>
			<span class="absolute -bottom-0.5 -left-0.5 size-4 border-b-2 border-l-2 border-primary"></span>
			<span class="absolute -right-0.5 -bottom-0.5 size-4 border-r-2 border-b-2 border-primary"></span>
		</div>
	{:else}
		<div class="absolute inset-0 bg-black/55"></div>
	{/if}
</div>
