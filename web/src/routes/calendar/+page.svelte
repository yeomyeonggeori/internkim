<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { calendarText } from './text';

	let iframeKey = $state(0);
	const text = createPageText(calendarText);

	function refreshCalendar() {
		iframeKey += 1;
	}
</script>

<svelte:head>
	<title>{text.title} · intern kim</title>
</svelte:head>

<main class="flex h-[calc(100svh-48px)] min-h-0 flex-col bg-background text-foreground">
	<div class="flex h-12 shrink-0 items-center justify-between border-b px-4">
		<div class="min-w-0">
			<h1 class="truncate text-sm font-medium">{text.title}</h1>
			<p class="truncate text-xs text-muted-foreground">{text.subtitle}</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={refreshCalendar}>
			<RefreshCwIcon />
		</Button>
	</div>

	{#key iframeKey}
		<iframe title={text.title} src="/calendar/embed" class="min-h-0 flex-1 border-0 bg-background"></iframe>
	{/key}
</main>
