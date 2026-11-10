<script lang="ts">
	import type { LinkPreview } from '$lib/messenger/messenger-api';
	import { linkPreviewOf } from '$lib/messenger/link-previews';
	import { isSupabaseConfigured } from '$lib/supabase';
	import ChannelLinkCard from './channel-link-card.svelte';

	let { url }: { url: string } = $props();

	let preview = $state<LinkPreview | null>(null);

	$effect(() => {
		if (!url || !isSupabaseConfigured()) return;
		let stillWanted = true;
		void linkPreviewOf(url)
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
	<ChannelLinkCard {preview} />
{/if}
