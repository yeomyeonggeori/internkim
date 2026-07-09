<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import { buildFlowPersonalScoreDetail, type FlowPersonalScorePeriod, type FlowPersonalScoreRow } from './flow-personal-score-detail-model';
	import type { FlowSummary } from './flow-types';
	import { flowText } from './text';

	type FlowReportText = typeof flowText.ko.report;

	type Props = {
		summary: FlowSummary | null;
		text: FlowReportText;
	};

	let { summary, text }: Props = $props();

	let detail = $derived(buildFlowPersonalScoreDetail(summary));

	function weekLabel(row: FlowPersonalScoreRow): string {
		if (row.periodIndex === 0) return text.currentWeekPeriodLabel;
		return text.weekPeriodLabel.replace('{count}', String(row.periodIndex));
	}

	function monthLabel(row: FlowPersonalScoreRow): string {
		if (row.periodIndex === 0) return text.currentMonthPeriodLabel;
		return text.monthPeriodLabel.replace('{count}', String(row.periodIndex));
	}

	function formatNumber(value: number): string {
		return new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 }).format(value);
	}

</script>

<section class="rounded-lg border bg-card p-4 shadow-sm">
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div class="space-y-1">
			<h2 class="text-lg font-semibold">{text.personalScoreTitle}</h2>
			<p class="text-sm text-muted-foreground">{text.personalScoreDescription}</p>
		</div>
		{#if detail}
			<div class="text-right">
				<div class="text-xs text-muted-foreground">{detail.memberName}</div>
				<div class="text-2xl font-semibold tabular-nums">
					{Math.round((detail.weekly.totalScore + detail.monthly.totalScore) / 2)}
				</div>
			</div>
		{/if}
	</div>

	{#if detail}
		<div class="mt-4 grid gap-4 lg:grid-cols-2">
			{@render scoreTable(text.weeklyScoreDetail, detail.weekly, weekLabel, text)}
			{@render scoreTable(text.monthlyScoreDetail, detail.monthly, monthLabel, text)}
		</div>
	{:else}
		<div class="mt-4 rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">{text.scoreEmpty}</div>
	{/if}
</section>

{#snippet scoreTable(title: string, period: FlowPersonalScorePeriod, labelForRow: (row: FlowPersonalScoreRow) => string, text: FlowReportText)}
	<div class="overflow-hidden rounded-md border">
		<div class="flex items-center justify-between border-b bg-muted/30 px-3 py-2">
			<div class="font-medium">{title}</div>
			<div class="text-sm tabular-nums text-muted-foreground">{text.scoreTotal} {period.totalScore}</div>
		</div>
		<div class="overflow-x-auto">
			<Table.Root class="w-full min-w-max table-auto">
				<Table.Header class="bg-muted/20">
					<Table.Row class="hover:bg-transparent">
						<Table.Head class="h-9 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.scorePeriod}</Table.Head>
						<Table.Head class="h-9 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.scoreCompletedDistance}</Table.Head>
						<Table.Head class="h-9 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.scoreCumulativeAverage}</Table.Head>
						<Table.Head class="h-9 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.scoreUnitScore}</Table.Head>
						<Table.Head class="h-9 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.scoreWeight}</Table.Head>
						<Table.Head class="h-9 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.scoreWeightedScore}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each period.rows as row}
						<Table.Row>
							<Table.Cell class="font-medium">{labelForRow(row)}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{formatNumber(row.completedDistance)}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{formatNumber(row.cumulativeAverage)}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{formatNumber(row.unitScore)}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{formatNumber(row.weight)}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{formatNumber(row.weightedScore)}</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	</div>
{/snippet}
