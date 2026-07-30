<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import FlowPersonalScoreDialog from '../flow-personal-score-dialog.svelte';
	import type { FlowSummary } from '../flow-types';
	import { flowText } from '../text';
	import type { FlowMemberScoreSection } from './flow-report-data';
	import FlowMemberScoreList from './flow-member-score-list.svelte';

	type FlowReportText = typeof flowText.ko.report;

	type Props = {
		section: FlowMemberScoreSection;
		summary: FlowSummary | null;
		text: FlowReportText;
	};

	let { section, summary, text }: Props = $props();

	function formatValue(value: number, unit: string): string {
		return `${value}${unit}`;
	}
</script>

<Card.Root class="flex h-full min-h-[28rem] min-w-0 w-full flex-col">
	<Card.Header class="pb-3">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
			<div class="min-w-0 space-y-1">
				<Card.Title class="text-base">{section.title}</Card.Title>
				{#if section.description}
					<Card.Description class="text-sm leading-snug">{section.description}</Card.Description>
				{/if}
				<Card.Description class="text-xs font-medium leading-snug tabular-nums">
					{section.teamAverageLabel}: {formatValue(section.averageValue, section.unit)}
				</Card.Description>
			</div>
			<FlowPersonalScoreDialog {summary} {text} />
		</div>
	</Card.Header>
	<Card.Content class="min-h-0 flex-1">
		{#if section.rows.length === 0}
			<div class="grid min-h-36 place-items-center rounded-md border border-dashed bg-muted/20 px-4 text-sm text-muted-foreground">
				{section.emptyLabel}
			</div>
		{:else}
			<FlowMemberScoreList {section} members={summary?.members ?? []} />
		{/if}
	</Card.Content>
</Card.Root>
