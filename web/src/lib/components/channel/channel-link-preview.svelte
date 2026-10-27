<script lang="ts">
	import { fetchLinkPreview, type LinkPreview } from '$lib/messenger/messenger-api';
	import { isSupabaseConfigured } from '$lib/supabase';
	import LoadingImage from '$lib/components/loading-image.svelte';
	import ChannelLinkVideo from './channel-link-video.svelte';

	let { url }: { url: string } = $props();

	let preview = $state<LinkPreview | null>(null);
	let failedImageURL = $state('');
	let failedVideoURL = $state('');
	const playableVideo = $derived(preview?.video && preview.video.url !== failedVideoURL ? preview.video : null);

	const cardClass =
		'flex w-full max-w-[min(70%,20rem)] flex-col self-start overflow-hidden rounded-lg border bg-card text-card-foreground no-underline group-data-[align=end]/message:self-end';

	$effect(() => {
		if (!url || !isSupabaseConfigured()) return;
		let stillWanted = true;
		void fetchLinkPreview(url)
			.catch(() => null)
			.then((found) => {
				if (stillWanted) preview = found;
			});
		return () => {
			stillWanted = false;
		};
	});
</script>

{#snippet summary(shown: LinkPreview)}
	<div class="grid gap-0.5 px-3 py-2">
		<span class="truncate text-xs text-muted-foreground">{shown.siteName}</span>
		<span class="line-clamp-2 text-sm font-medium">{shown.title}</span>
		{#if shown.description}
			<span class="line-clamp-2 text-xs text-muted-foreground">{shown.description}</span>
		{/if}
	</div>
{/snippet}

{#if preview && playableVideo}
	<div class={cardClass}>
		<ChannelLinkVideo
			video={playableVideo}
			posterURL={preview.imageURL}
			title={preview.title}
			onFailed={() => (failedVideoURL = playableVideo.url)}
		/>
		<a href={preview.url} target="_blank" rel="noreferrer" class="no-underline transition-colors hover:bg-accent">
			{@render summary(preview)}
		</a>
	</div>
{:else if preview}
	<a href={preview.url} target="_blank" rel="noreferrer" class="{cardClass} transition-colors hover:bg-accent">
		{#if preview.imageURL && preview.imageURL !== failedImageURL}
			<LoadingImage
				src={preview.imageURL}
				alt=""
				loading="lazy"
				fill
				class="aspect-[1.91/1] w-full"
				imageClass="object-cover"
				onFailed={() => (failedImageURL = preview?.imageURL ?? '')}
			/>
		{/if}
		{@render summary(preview)}
	</a>
{/if}
