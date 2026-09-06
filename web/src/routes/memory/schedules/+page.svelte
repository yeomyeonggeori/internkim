<script lang="ts">
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import { Button } from '$lib/components/ui/button';
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MemoryScheduleList from '../memory-schedule-list.svelte';
	import { memoryText } from '../text';

	const text = createPageText(memoryText);
</script>

<svelte:head><title>{text.scheduleTab} · {text.pageTitle}</title></svelte:head>

<main class="mx-auto flex w-full min-w-0 max-w-6xl flex-1 flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
	<header class="flex flex-col items-start gap-3">
		<Button href={appNavigation.link('/memory/')} variant="ghost" size="sm"><ArrowLeftIcon data-icon="inline-start" />{text.factListTab}</Button>
		<h1 class="text-xl font-semibold tracking-tight">{text.scheduleTab}</h1>
	</header>
	<section aria-label={text.scheduleTab}><MemoryScheduleList {text} /></section>
</main>
