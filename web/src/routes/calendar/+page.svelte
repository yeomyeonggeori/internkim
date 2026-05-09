<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';

	let iframeKey = $state(0);

	function refreshCalendar() {
		iframeKey += 1;
	}
</script>

<svelte:head>
	<title>Calendar · intern kim</title>
</svelte:head>

<main class="flex h-[calc(100svh-48px)] min-h-0 flex-col bg-background text-foreground">
	<div class="flex h-12 shrink-0 items-center justify-between border-b px-4">
		<div class="min-w-0">
			<h1 class="truncate text-sm font-medium">Calendar</h1>
			<p class="truncate text-xs text-muted-foreground">Team schedule</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label="Refresh calendar" onclick={refreshCalendar}>
			<RefreshCwIcon />
		</Button>
	</div>

	{#key iframeKey}
		<iframe title="Intern Kim Calendar" src="/calendar/embed" class="min-h-0 flex-1 border-0 bg-background"></iframe>
	{/key}
</main>
