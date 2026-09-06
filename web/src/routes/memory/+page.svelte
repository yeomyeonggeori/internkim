<script lang="ts">
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MemoryFactBrowser from './memory-fact-browser.svelte';
	import MemoryGraphPanel from './memory-graph-panel.svelte';
	import MemoryScheduleList from './memory-schedule-list.svelte';
	import { memoryText } from './text';

	let activeTab = $state('facts');
	const text = createPageText(memoryText);
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>


<main class="mx-auto grid min-h-[calc(100svh-48px)] w-full max-w-6xl flex-1 content-start gap-5 overflow-x-hidden px-4 py-5 sm:px-6 lg:px-8">
	<header class="grid gap-1">
		<h1 class="text-xl font-semibold tracking-tight">{text.title}</h1>
		<p class="max-w-2xl text-sm text-muted-foreground">{text.description}</p>
	</header>
	<UnderlineTabs.Root bind:value={activeTab} class="min-w-0 gap-6">
		<UnderlineTabs.List>
			<UnderlineTabs.Trigger value="facts">{text.factListTab}</UnderlineTabs.Trigger>
			<UnderlineTabs.Trigger value="search">{text.searchTab}</UnderlineTabs.Trigger>
			<UnderlineTabs.Trigger value="graph">{text.graphTab}</UnderlineTabs.Trigger>
			<UnderlineTabs.Trigger value="schedules">{text.scheduleTab}</UnderlineTabs.Trigger>
		</UnderlineTabs.List>
		<UnderlineTabs.Content value="facts" class="grid min-w-0 gap-5">
			<MemoryFactBrowser {text} />
		</UnderlineTabs.Content>
		<UnderlineTabs.Content value="search" class="grid min-w-0 gap-5"><MemoryFactBrowser {text} mode="search" /></UnderlineTabs.Content>
		<UnderlineTabs.Content value="graph" class="grid min-w-0 gap-5">
			<MemoryGraphPanel {text} />
		</UnderlineTabs.Content>
		<UnderlineTabs.Content value="schedules" class="grid min-w-0 gap-5">
			<MemoryScheduleList {text} />
		</UnderlineTabs.Content>
	</UnderlineTabs.Root>
</main>
