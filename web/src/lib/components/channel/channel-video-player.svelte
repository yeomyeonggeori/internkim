<script lang="ts" module>
	export type VideoPlayerVariant = 'message' | 'viewer';
</script>

<script lang="ts">
	import Maximize2Icon from '@lucide/svelte/icons/maximize-2';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { videoPlaybackHandoff } from './video-playback-handoff';

	let {
		source,
		identity,
		width,
		height,
		variant,
		onOpen,
		onFailed
	}: {
		source: string;
		identity: string;
		width?: number;
		height?: number;
		variant: VideoPlayerVariant;
		onOpen?: () => void;
		onFailed?: () => void;
	} = $props();

	const text = createPageText(channelText);

	const aspectRatio = $derived(width && height ? `${width} / ${height}` : '16 / 9');
	let video: HTMLVideoElement | undefined;
	let resumeAtSeconds = 0;

	function renewKeepingPosition(): void {
		resumeAtSeconds = video?.currentTime ?? 0;
		onFailed?.();
	}

	function resumeAfterRenewal(): void {
		if (!video || resumeAtSeconds === 0) return;
		video.currentTime = resumeAtSeconds;
		resumeAtSeconds = 0;
	}
</script>

<div
	data-video-player={variant}
	class={variant === 'message'
		? 'relative w-[26rem] max-w-full overflow-hidden rounded-lg bg-black'
		: 'relative z-10 w-[min(calc(100vw-3rem),64rem)]'}
>
	<video
		bind:this={video}
		src={`${source}#t=0.001`}
		controls
		preload="metadata"
		playsinline
		onerror={renewKeepingPosition}
		onloadedmetadata={resumeAfterRenewal}
		use:videoPlaybackHandoff={identity}
		style:aspect-ratio={aspectRatio}
		class={variant === 'message' ? 'size-full object-contain' : 'max-h-[calc(100dvh-10rem)] size-full rounded-md bg-black object-contain'}
	></video>
	{#if onOpen}
		<button
			type="button"
			class="absolute top-2 left-2 flex size-8 items-center justify-center rounded-full bg-black/60 text-white backdrop-blur transition hover:bg-black/80"
			aria-label={text.openVideo}
			onclick={onOpen}
		>
			<Maximize2Icon class="size-4" />
		</button>
	{/if}
</div>
