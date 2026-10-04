<script lang="ts">
	import CalendarClockIcon from '@lucide/svelte/icons/calendar-clock';
	import { Button } from '$lib/components/ui/button';
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MemoryFactBrowser from './memory-fact-browser.svelte';
	import { memoryText } from './text';

	const text = createPageText(memoryText);
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>


<main class="mx-auto flex w-full min-w-0 max-w-7xl flex-1 flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
	<header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div class="flex min-w-0 flex-col gap-1.5">
			<h1 class="text-xl font-semibold tracking-tight">{text.title}</h1>
			<p class="max-w-2xl text-sm text-muted-foreground">{text.description}</p>
		</div>
		<nav class="flex shrink-0 flex-wrap gap-2" aria-label={text.relatedViews}>
			<Button href={appNavigation.link('/memory/schedules/')} variant="outline" size="sm"><CalendarClockIcon data-icon="inline-start" />{text.scheduleTab}</Button>
		</nav>
	</header>
	<MemoryFactBrowser {text} />
</main>
