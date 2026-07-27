<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
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
	};

	let {
		summary,
		text,
		isLoading,
		selectWeek,
		selectCurrentWeek
	}: Props = $props();
</script>

<header class="flex min-w-0 flex-col gap-4 md:flex-row md:items-center md:justify-end">
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
	</div>
</header>
