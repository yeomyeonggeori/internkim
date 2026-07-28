<script lang="ts">
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { ConfirmDeleteDialog } from '$lib/components/ui/confirm-delete-dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MemoryFactList from './memory-fact-list.svelte';
	import MemoryGraphPanel from './memory-graph-panel.svelte';
	import MemoryScheduleList from './memory-schedule-list.svelte';
	import { memoryText } from './text';

	let activeTab = $state('facts');
	const text = createPageText(memoryText);
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="grid min-h-[calc(100svh-48px)] w-full flex-1 content-start gap-5 overflow-x-hidden px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<UnderlineTabs.Root bind:value={activeTab} class="min-w-0 gap-6">
		<UnderlineTabs.List>
			<UnderlineTabs.Trigger value="facts">{text.factListTab}</UnderlineTabs.Trigger>
			<UnderlineTabs.Trigger value="graph">{text.graphTab}</UnderlineTabs.Trigger>
			<UnderlineTabs.Trigger value="schedules">{text.scheduleTab}</UnderlineTabs.Trigger>
		</UnderlineTabs.List>
		<UnderlineTabs.Content value="facts" class="grid min-w-0 gap-5">
			<MemoryFactList {text} />
		</UnderlineTabs.Content>
		<UnderlineTabs.Content value="graph" class="grid min-w-0 gap-5">
			<MemoryGraphPanel {text} />
		</UnderlineTabs.Content>
		<UnderlineTabs.Content value="schedules" class="grid min-w-0 gap-5">
			<MemoryScheduleList {text} />
		</UnderlineTabs.Content>
	</UnderlineTabs.Root>
</main>

<ConfirmDeleteDialog />
