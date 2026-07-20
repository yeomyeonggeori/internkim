<script lang="ts">
	import { buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import { cn } from '$lib/utils';
	import ChartNoAxesColumnIncreasingIcon from '@lucide/svelte/icons/chart-no-axes-column-increasing';
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
		if (row.periodIndex === 1) return text.weekPeriodSingularLabel;
		return text.weekPeriodLabel.replace('{count}', String(row.periodIndex));
	}

	function monthLabel(row: FlowPersonalScoreRow): string {
		if (row.periodIndex === 0) return text.currentMonthPeriodLabel;
		if (row.periodIndex === 1) return text.monthPeriodSingularLabel;
		return text.monthPeriodLabel.replace('{count}', String(row.periodIndex));
	}

	function formatNumber(value: number): string {
		return new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 }).format(value);
	}

	function formatScoreNumber(value: number): string {
		return new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
	}

	function personalScoreTitle(): string {
		if (!detail) return text.personalScoreTitle;
		return text.personalScoreMemberTitle.replace('{name}', detail.memberName);
	}

	function overallScore(): number {
		if (!detail) return 0;
		return Math.round((detail.weekly.totalScore + detail.monthly.totalScore) / 2);
	}
</script>

<Dialog.Root>
	<Dialog.Trigger class={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'shrink-0 gap-2')}>
		<ChartNoAxesColumnIncreasingIcon class="size-4" />
		{text.personalScoreAction}
	</Dialog.Trigger>
	<Dialog.Content closeLabel={text.personalScoreClose} class="max-h-[calc(100vh-2rem)] overflow-y-auto p-0 sm:max-w-[calc(100vw-2rem)] xl:max-w-6xl">
		<Dialog.Header class="border-b px-6 py-5 pr-12">
			<div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
				<div class="space-y-1.5">
					<Dialog.Title class="text-xl">{personalScoreTitle()}</Dialog.Title>
					<Dialog.Description>{text.personalScoreDescription}</Dialog.Description>
				</div>
				{#if detail}
					<div class="flex flex-wrap gap-x-5 gap-y-2 text-sm">
						<div class="flex items-baseline gap-1.5">
							<span class="text-muted-foreground">{text.personalScoreOverallLabel}</span>
							<span class="text-xl font-semibold tabular-nums">{overallScore()}{text.scoreUnit}</span>
						</div>
						<div class="flex items-baseline gap-1.5">
							<span class="text-muted-foreground">{text.weeklyScoreLabel}</span>
							<span class="font-medium tabular-nums">{detail.weekly.totalScore}{text.scoreUnit}</span>
						</div>
						<div class="flex items-baseline gap-1.5">
							<span class="text-muted-foreground">{text.monthlyScoreLabel}</span>
							<span class="font-medium tabular-nums">{detail.monthly.totalScore}{text.scoreUnit}</span>
						</div>
					</div>
				{/if}
			</div>
		</Dialog.Header>

		<div class="p-6">
			{#if detail}
				<div class="grid gap-4 lg:grid-cols-2">
					{@render scoreTable(text.weeklyScoreDetail, detail.weekly, weekLabel, text)}
					{@render scoreTable(text.monthlyScoreDetail, detail.monthly, monthLabel, text)}
				</div>
			{:else}
				<div class="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">{text.scoreEmpty}</div>
			{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>

{#snippet scoreTable(title: string, period: FlowPersonalScorePeriod, labelForRow: (row: FlowPersonalScoreRow) => string, text: FlowReportText)}
	<div class="overflow-hidden rounded-md border">
		<div class="flex items-center justify-between border-b bg-muted/30 px-3 py-2">
			<div class="font-medium">{title}</div>
			<div class="font-mono text-sm tabular-nums text-muted-foreground">{text.scoreTotal} {period.totalScore}</div>
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
							<Table.Cell class="text-right font-mono tabular-nums">{formatNumber(row.completedDistance)}</Table.Cell>
							<Table.Cell class="text-right font-mono tabular-nums">{formatScoreNumber(row.cumulativeAverage)}</Table.Cell>
							<Table.Cell class="text-right font-mono tabular-nums">{formatScoreNumber(row.unitScore)}</Table.Cell>
							<Table.Cell class="text-right font-mono tabular-nums">{formatNumber(row.weight)}</Table.Cell>
							<Table.Cell class="text-right font-mono tabular-nums">{formatScoreNumber(row.weightedScore)}</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	</div>
{/snippet}
