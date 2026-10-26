<script lang="ts" module>
	export type VideoPlayerVariant = 'message' | 'viewer';
</script>

<script lang="ts">
	import Maximize2Icon from '@lucide/svelte/icons/maximize-2';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	let {
		source,
		width,
		height,
		variant,
		onOpen
	}: {
		source: string;
		width?: number;
		height?: number;
		variant: VideoPlayerVariant;
		onOpen?: () => void;
	} = $props();

	const text = createPageText(channelText);

	const aspectRatio = $derived(width && height ? `${width} / ${height}` : '16 / 9');
</script>

<div
	data-video-player={variant}
	class={variant === 'message'
		? 'relative w-[26rem] max-w-full overflow-hidden rounded-lg bg-black'
		: 'relative z-10 w-[min(calc(100vw-3rem),64rem)]'}
>
	<video
		src={source}
		controls
		preload="metadata"
		playsinline
		style:aspect-ratio={aspectRatio}
		class={variant === 'message' ? 'size-full object-contain' : 'max-h-[calc(100dvh-10rem)] size-full rounded-md bg-black object-contain'}
	></video>
	{#if onOpen}
		<button
			type="button"
			class="absolute top-2 right-2 flex size-8 items-center justify-center rounded-full bg-black/60 text-white backdrop-blur transition hover:bg-black/80"
			aria-label={text.openVideo}
			onclick={onOpen}
		>
			<Maximize2Icon class="size-4" />
		</button>
	{/if}
</div>
