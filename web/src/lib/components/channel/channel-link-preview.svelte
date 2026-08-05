<script lang="ts">
	import { fetchLinkPreview, type LinkPreview } from '$lib/messenger/messenger-api';
	import { isSupabaseConfigured } from '$lib/supabase';

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
		class="flex w-fit max-w-[min(80%,32rem)] flex-col overflow-hidden rounded-lg border bg-card text-card-foreground no-underline transition-colors hover:bg-accent"
	>
		{#if preview.imageDataURL}
			<img src={preview.imageDataURL} alt="" loading="lazy" decoding="async" class="h-32 w-full object-cover" />
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
