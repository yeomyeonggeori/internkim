<script lang="ts">
	import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
	import { ConfirmDeleteDialog } from '$lib/components/ui/confirm-delete-dialog';
	import NetworkIcon from '@lucide/svelte/icons/network';
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
	<section class="flex min-w-0 flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h1 class="flex items-center gap-2 text-xl font-semibold">
				<NetworkIcon class="size-5 text-teal-700" />
				{text.title}
			</h1>
			<p class="mt-1 max-w-full text-sm text-muted-foreground">{text.description}</p>
		</div>
	</section>

	<Tabs bind:value={activeTab} class="min-w-0 gap-6">
		<TabsList variant="line" class="memory-tabs-list w-full justify-start gap-10 border-b p-0">
			<TabsTrigger value="facts" class="memory-tab-trigger">{text.factListTab}</TabsTrigger>
			<TabsTrigger value="graph" class="memory-tab-trigger">{text.graphTab}</TabsTrigger>
			<TabsTrigger value="schedules" class="memory-tab-trigger">{text.scheduleTab}</TabsTrigger>
		</TabsList>
		<TabsContent value="facts" class="grid min-w-0 gap-5">
			<MemoryFactList {text} />
		</TabsContent>
		<TabsContent value="graph" class="grid min-w-0 gap-5">
			<MemoryGraphPanel {text} />
		</TabsContent>
		<TabsContent value="schedules" class="grid min-w-0 gap-5">
			<MemoryScheduleList {text} />
		</TabsContent>
	</Tabs>
</main>

<ConfirmDeleteDialog />

<style>
	:global(.memory-tabs-list) {
		height: 3.5rem;
	}

	:global(.memory-tab-trigger) {
		height: 3.5rem;
		flex: none;
		border-radius: 0;
		padding: 0;
		font-size: 1.125rem;
		font-weight: 700;
		color: hsl(var(--muted-foreground));
	}

	:global(.memory-tab-trigger:hover) {
		color: hsl(var(--foreground));
	}

	:global(.memory-tabs-list .memory-tab-trigger[data-active]),
	:global(.memory-tabs-list .memory-tab-trigger[aria-selected='true']) {
		color: var(--primary);
	}

	:global(.memory-tab-trigger::after) {
		bottom: -1px;
		background-color: var(--primary);
		height: 2px;
	}

	:global(.memory-tabs-list .memory-tab-trigger[data-active]::after),
	:global(.memory-tabs-list .memory-tab-trigger[aria-selected='true']::after) {
		opacity: 1;
	}
</style>
