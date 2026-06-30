<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import FlowWeekSelector from './flow-week-selector.svelte';
	import { flowText } from './text';
	import type { FlowSummary } from './flow-types';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		summary: FlowSummary | null;
		text: FlowPageText;
		isLoading: boolean;
		selectWeek: (week: string) => void;
		selectCurrentWeek: () => void;
		refreshCurrentWeek: () => void;
	};

	let {
		summary,
		text,
		isLoading,
		selectWeek,
		selectCurrentWeek,
		refreshCurrentWeek
	}: Props = $props();
</script>

<header class="flex min-w-0 flex-col gap-4 border-b pb-5 md:flex-row md:items-end md:justify-between">
	<div class="min-w-0 space-y-1">
		<p class="text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.product}</p>
		<h1 class="text-2xl font-semibold">{text.title}</h1>
		<p class="text-sm text-muted-foreground">{text.description}</p>
	</div>
	<div class="flex flex-wrap items-center gap-2">
		<Button variant="outline" size="sm" onclick={() => selectWeek(summary?.week.previous ?? '')} disabled={!summary || isLoading}>
			<ChevronLeftIcon />
			{text.previousWeek}
		</Button>
		<FlowWeekSelector
			week={summary?.week}
			disabled={!summary || isLoading}
			selectDateLabel={text.selectWeekDate}
			currentWeekLabel={text.currentWeekAction}
			onSelectWeek={selectWeek}
			onSelectCurrentWeek={selectCurrentWeek}
		/>
		<Button variant="outline" size="sm" onclick={() => selectWeek(summary?.week.next ?? '')} disabled={!summary || isLoading}>
			{text.nextWeek}
			<ChevronRightIcon />
		</Button>
		<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={refreshCurrentWeek} disabled={isLoading}>
			<RefreshCwIcon class={isLoading ? 'animate-spin' : ''} />
		</Button>
	</div>
</header>
