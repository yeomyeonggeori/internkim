<script lang="ts">
	import { fetchLinkPreview, type LinkPreview } from '$lib/messenger/messenger-api';
	import { isSupabaseConfigured } from '$lib/supabase';
	import LoadingImage from '$lib/components/loading-image.svelte';

	let { url }: { url: string } = $props();

	let preview = $state<LinkPreview | null>(null);

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

{#if preview}
	<a
		href={preview.url}
		target="_blank"
		rel="noreferrer"
		class="flex w-full max-w-[min(70%,20rem)] flex-col self-start overflow-hidden rounded-lg border bg-card text-card-foreground no-underline transition-colors group-data-[align=end]/message:self-end hover:bg-accent"
	>
		{#if preview.imageURL}
			<LoadingImage
				src={preview.imageURL}
				alt=""
				loading="lazy"
				fill
				class="aspect-[1.91/1] w-full"
				imageClass="object-cover"
			/>
		{/if}
		<div class="grid gap-0.5 px-3 py-2">
			<span class="truncate text-xs text-muted-foreground">{preview.siteName}</span>
			<span class="line-clamp-2 text-sm font-medium">{preview.title}</span>
			{#if preview.description}
				<span class="line-clamp-2 text-xs text-muted-foreground">{preview.description}</span>
			{/if}
		</div>
	</a>
{/if}
