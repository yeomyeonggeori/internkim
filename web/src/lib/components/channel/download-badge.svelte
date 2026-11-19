<script lang="ts">
	import DownloadIndicator from '$lib/files/download-indicator.svelte';
	import { downloadProgress } from '$lib/files/download-progress.svelte';

	let { identity }: { identity: string } = $props();
</script>

{#if downloadProgress.stateOf(identity)}
	<span
		data-download-badge
		class="pointer-events-none absolute right-2 bottom-2 z-10 flex size-9 items-center justify-center rounded-full bg-black/60 text-white"
	>
		<DownloadIndicator {identity} />
	</span>
{/if}
