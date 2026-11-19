<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import type { Snippet } from 'svelte';
	import { ProgressRing } from '$lib/components/ui/progress-ring/index.js';
	import { downloadText } from '$lib/i18n/download-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { downloadProgress } from './download-progress.svelte';

	let { identity, class: className, idle }: { identity: string; class?: string; idle?: Snippet } = $props();

	const text = createPageText(downloadText);
	const shown = $derived(downloadProgress.stateOf(identity));
</script>

{#if shown?.phase === 'running'}
	<ProgressRing
		class={className}
		value={shown.progress.fraction === null ? null : shown.progress.fraction * 100}
		aria-label={text.receiving}
	/>
{:else if shown?.phase === 'finished'}
	<CheckIcon role="img" aria-label={text.received} class="size-5" />
{:else if idle}
	{@render idle()}
{/if}
