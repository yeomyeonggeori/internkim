<script lang="ts">
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import { onMount } from 'svelte';
	import { isBrowserOutdated } from '$lib/browser-support';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(appShellText);
	let isOutdated = $state(false);

	onMount(() => {
		isOutdated = isBrowserOutdated();
	});
</script>

{#if isOutdated}
	<div
		role="status"
		class="flex shrink-0 items-center gap-2.5 border-b border-warning/30 bg-warning-subtle px-4 py-1.5 text-sm text-warning-subtle-foreground"
	>
		<TriangleAlertIcon class="size-4 shrink-0" aria-hidden="true" />
		<p class="min-w-0 flex-1 break-keep">
			<span class="font-medium">{text.browserOutdated}</span>
			<span class="opacity-80">{text.browserUpdateAdvice}</span>
		</p>
	</div>
{/if}
