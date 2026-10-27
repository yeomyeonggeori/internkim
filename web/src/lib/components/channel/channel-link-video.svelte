<script lang="ts">
	import PlayIcon from '@lucide/svelte/icons/play';
	import type { LinkVideo } from '$lib/messenger/messenger-api';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LoadingImage from '$lib/components/loading-image.svelte';

	let {
		video,
		posterURL,
		title,
		onFailed
	}: { video: LinkVideo; posterURL: string; title: string; onFailed: () => void } = $props();

	const text = createPageText(channelText);
	let isPlaying = $state(false);
	let hasPosterFailed = $state(false);
	const widthPerHeight = $derived(video.width && video.height ? video.width / video.height : 16 / 9);
</script>

<div class="flex w-full justify-center bg-black">
	<div
		data-link-video={video.kind}
		class="relative"
		style:aspect-ratio={widthPerHeight}
		style:width={`min(100%, calc(min(60vh, 26rem) * ${widthPerHeight}))`}
	>
		{#if !isPlaying}
			<button
				type="button"
				aria-label={`${title} ${text.playLinkVideo}`}
				class="group/play absolute inset-0 flex items-center justify-center"
				onclick={() => (isPlaying = true)}
			>
				{#if posterURL && !hasPosterFailed}
					<LoadingImage
						src={posterURL}
						alt=""
						loading="lazy"
						fill
						class="absolute inset-0"
						imageClass="object-cover"
						onFailed={() => (hasPosterFailed = true)}
					/>
				{/if}
				<span class="relative flex size-12 items-center justify-center rounded-full bg-black/60 text-white backdrop-blur transition group-hover/play:bg-black/80">
					<PlayIcon aria-hidden="true" class="size-5 fill-current" />
				</span>
			</button>
		{:else if video.kind === 'file'}
			<video src={video.url} controls autoplay playsinline class="size-full object-contain" onerror={onFailed}></video>
		{:else}
			<iframe
				src={video.url}
				{title}
				allow="autoplay; encrypted-media; fullscreen; picture-in-picture"
				allowfullscreen
				referrerpolicy="strict-origin-when-cross-origin"
				sandbox="allow-scripts allow-same-origin allow-presentation allow-popups"
				class="size-full border-0"
			></iframe>
		{/if}
	</div>
</div>
