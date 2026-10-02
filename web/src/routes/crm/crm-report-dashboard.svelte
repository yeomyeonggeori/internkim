<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Chart from '$lib/components/ui/chart';
	import * as HoverCard from '$lib/components/ui/hover-card';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import { BarChart } from 'layerchart';
	import ColorMarker from '$lib/components/color-marker.svelte';
	import PersonChip from '$lib/components/person-chip.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';
	import CRMViewCurrencySelect from './crm-view-currency-select.svelte';
	import CRMTableColumnHeader from './crm-table-column-header.svelte';
	import { nextSortState, sortRows, type CRMSortComparators, type CRMSortState } from './crm-table-sort';
	import { buildCRMReportPeriodBounds, currentCRMDate } from './crm-date';
	import { crmLabel } from './crm-labels';
	import { dealStageIcon } from './crm-status-icons';
	import type {
		CRMMoneyTotals,
		CRMNextAction,
		CRMOpportunity,
		CRMOrganization,
		CRMPipeline,
		CRMPipelineStage
	} from './crm-types';
	import { formatViewMoneyTotals } from './crm-money';
	import { crmViewCurrency } from './crm-view-currency.svelte';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import { formatCRMDate, opportunityStageLabel } from './crm-view-model';
	import {
		crmReportRange,
		monthBuckets,
		opportunitiesInRange,
		ownerRows,
		periodOutcome,
		pipelineRows,
		quietAccounts,
		stageRows,
		type CRMOwnerRow,
		type CRMQuietAccount,
		type CRMReportPeriod
	} from './crm-report-model';
	import type { CRMText } from './text';

	type Props = {
		organizations: CRMOrganization[];
		opportunities: CRMOpportunity[];
		pipelines: CRMPipeline[];
		nextActions: CRMNextAction[];
		stages: CRMPipelineStage[];
		currencyCatalogue: CurrencyCatalogue;
		companyBaseCurrency: string;
		sourceCurrencies: string[];
		text: CRMText;
		onOpenOrganization: (organizationID: string) => void;
	};

	let {
		organizations,
		opportunities,
		pipelines,
		nextActions,
		stages,
		currencyCatalogue,
		companyBaseCurrency,
		sourceCurrencies,
		text,
		onOpenOrganization
	}: Props = $props();

	const bounds = buildCRMReportPeriodBounds();
	const quietAccountLimit = 8;
	const overdueContactDays = 30;

	let period = $state<CRMReportPeriod>('all');
	let ownerSort = $state<CRMSortState | null>({ key: 'openAmount', direction: 'descending' });
	let quietSort = $state<CRMSortState | null>({ key: 'days', direction: 'descending' });

	const quarterLabel = $derived.by(() => {
		const [year, month] = bounds.quarterStart.split('-').map(Number);
		return text.reportPeriodQuarter
			.replace('{year}', String(year))
			.replace('{quarter}', String(Math.floor(((month ?? 1) - 1) / 3) + 1));
	});
	const periodLabel = $derived(
		period === 'quarter' ? quarterLabel : period === 'next_90_days' ? text.reportPeriodNext90Days : text.reportPeriodAll
	);

	const convert = $derived((value: number | undefined, currency: string) =>
		value === undefined ? 0 : crmViewCurrency.viewAmount(value, currency).value
	);

	const range = $derived(crmReportRange(period, bounds));
	const inPeriod = $derived(opportunitiesInRange(opportunities, range));
	const outcome = $derived(periodOutcome(inPeriod, stages, convert));
	const months = $derived(monthBuckets(inPeriod, stages, range, convert));
	const stageBars = $derived(stageRows(inPeriod, stages, (stage) => opportunityStageLabel(stages, stage, text), convert));
	const pipelineBars = $derived(
		pipelineRows(inPeriod, organizations, pipelines, (kind) => crmLabel(text.progressKinds, kind), convert)
	);
	const quiet = $derived(quietAccounts(organizations, currentCRMDate(), quietAccountLimit));
	const owners = $derived(ownerRows(inPeriod, organizations, nextActions, stages, convert));

	const ownerComparators: CRMSortComparators<CRMOwnerRow> = {
		name: (owner) => owner.name,
		organizationCount: (owner) => owner.organizationCount,
		openCount: (owner) => owner.openCount,
		openAmount: (owner) => owner.openAmount,
		wonAmount: (owner) => owner.wonAmount,
		missingActionCount: (owner) => owner.missingActionCount
	};
	const sortedOwners = $derived(sortRows(owners, ownerSort, ownerComparators));

	const quietComparators: CRMSortComparators<CRMQuietAccount> = {
		name: (account) => account.name,
		owner: (account) => account.ownerName,
		lastContact: (account) => account.lastContactDate,
		days: (account) => account.daysSinceContact
	};
	const sortedQuiet = $derived(sortRows(quiet, quietSort, quietComparators));

	function toggleQuietSort(key: string): void {
		quietSort = nextSortState(quietSort, key);
	}

	function quietAriaSort(key: string): 'ascending' | 'descending' | 'none' {
		return quietSort?.key === key ? quietSort.direction : 'none';
	}

	function handleQuietKeydown(event: KeyboardEvent, organizationID: string): void {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		onOpenOrganization(organizationID);
	}

	const chartData = $derived(months.map((bucket, index) => ({ ...bucket, index })));
	const chartConfig = $derived<Chart.ChartConfig>({
		openAmount: { label: text.openProgress, color: 'var(--color-muted-foreground)' },
		wonAmount: { label: text.won, color: 'var(--color-foreground)' }
	});
	const chartSeries = $derived([
		{ key: 'openAmount', label: text.openProgress, color: 'var(--color-muted-foreground)' },
		{ key: 'wonAmount', label: text.won, color: 'var(--color-foreground)' }
	]);
	const axisLabelIndexes = $derived(evenlySpacedIndexes(chartData.length, 8));

	const stageBarMaximum = $derived(Math.max(1, ...stageBars.map((row) => row.count)));
	const pipelineBarMaximum = $derived(Math.max(1, ...pipelineBars.map((row) => row.count)));

	function toggleOwnerSort(key: string): void {
		ownerSort = nextSortState(ownerSort, key);
	}

	function ariaSort(key: string): 'ascending' | 'descending' | 'none' {
		return ownerSort?.key === key ? ownerSort.direction : 'none';
	}

	function money(totals: CRMMoneyTotals): string {
		return formatViewMoneyTotals(totals, currencyCatalogue, crmViewCurrency, text.noValue, currentLocale.value);
	}

	function monthLabel(month: string): string {
		return month.replace('-', '. ');
	}

	function evenlySpacedIndexes(count: number, maximumCount: number): number[] {
		if (count <= maximumCount) return Array.from({ length: count }, (_, index) => index);
		return Array.from({ length: maximumCount }, (_, index) =>
			Math.round((index * (count - 1)) / (maximumCount - 1))
		);
	}

	function formatAxisLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		if (!axisLabelIndexes.includes(index)) return '';
		return chartData[index] ? monthLabel(chartData[index].month) : '';
	}

	function formatTooltipLabel(value: unknown): string {
		const index = typeof value === 'number' ? value : Number(value);
		return chartData[index] ? monthLabel(chartData[index].month) : String(value);
	}

	function formatTooltipValue(value: unknown, name: unknown): string {
		const index = chartData.findIndex((point) => point.openAmount === value || point.wonAmount === value);
		const bucket = chartData[index];
		if (!bucket) return String(value);
		return money(name === text.won ? bucket.wonTotals : bucket.openTotals);
	}

	function winRateLabel(rate: number | null): string {
		return rate === null ? text.noValue : `${Math.round(rate * 100)}%`;
	}
</script>

{#snippet moneyValue(totals: CRMMoneyTotals, label: string)}
	{@const breakdown = currencyCatalogue.filter((entry) => totals[entry.code] !== undefined)}
	{#if breakdown.length > 1}
		<HoverCard.Root openDelay={150}>
			<HoverCard.Trigger>
				{#snippet child({ props })}
					<button
						{...props}
						type="button"
						class="rounded-sm border-0 bg-transparent p-0 text-left font-[inherit] text-inherit focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
						aria-label={`${label}: ${money(totals)}`}
					>
						{money(totals)}
					</button>
				{/snippet}
			</HoverCard.Trigger>
			<HoverCard.Content side="top" align="center" sideOffset={8} class="w-48 p-3">
				<p class="mb-2 text-xs font-medium">{label}</p>
				<div class="grid gap-1.5">
					{#each breakdown as entry (entry.code)}
						<div class="flex items-center justify-between gap-4 text-xs">
							<span class="text-muted-foreground">{entry.code}</span>
							<span class="font-medium tabular-nums">{money({ [entry.code]: totals[entry.code] })}</span>
						</div>
					{/each}
				</div>
			</HoverCard.Content>
		</HoverCard.Root>
	{:else}
		{money(totals)}
	{/if}
{/snippet}

<div class="grid min-w-0 gap-4" data-crm-report>
	<div class="flex min-w-0 flex-wrap items-center gap-2">
		<Select.Root type="single" value={period} onValueChange={(value) => (period = value as CRMReportPeriod)}>
			<Select.Trigger class="w-44" aria-label={text.reportPeriod}>{periodLabel}</Select.Trigger>
			<Select.Content>
				<Select.Item value="quarter" label={quarterLabel}>{quarterLabel}</Select.Item>
				<Select.Item value="next_90_days" label={text.reportPeriodNext90Days}>{text.reportPeriodNext90Days}</Select.Item>
				<Select.Item value="all" label={text.reportPeriodAll}>{text.reportPeriodAll}</Select.Item>
			</Select.Content>
		</Select.Root>
		<CRMViewCurrencySelect {text} {currencyCatalogue} {companyBaseCurrency} {sourceCurrencies} />
	</div>

	<section class="min-w-0" data-crm-report-card="outcome">
		<Card.Root class="grid min-w-0 grid-cols-1 gap-px bg-border py-0 sm:grid-cols-3">
			<div class="flex min-w-0 flex-col gap-3 bg-card p-4">
				<h2 class="text-xs font-medium text-muted-foreground">{text.openDealAmount}</h2>
				<p class="min-w-0 text-2xl font-semibold tracking-tight tabular-nums">
					{@render moneyValue(outcome.openTotals, text.openDealAmount)}
				</p>
			</div>
			<div class="flex min-w-0 flex-col gap-3 bg-card p-4">
				<h2 class="text-xs font-medium text-muted-foreground">{text.wonValue}</h2>
				<p class="min-w-0 text-2xl font-semibold tracking-tight tabular-nums">
					{@render moneyValue(outcome.wonTotals, text.wonValue)}
				</p>
			</div>
			<div class="flex min-w-0 flex-col gap-3 bg-card p-4">
				<h2 class="text-xs font-medium text-muted-foreground">{text.winRate}</h2>
				<p class="min-w-0 text-2xl font-semibold tracking-tight tabular-nums">{winRateLabel(outcome.winRate)}</p>
			</div>
		</Card.Root>
	</section>

	<Card.Root class="min-w-0" data-crm-report-card="monthly">
		<Card.Header class="flex flex-wrap items-center justify-between gap-3">
			<Card.Title class="text-base">{text.monthlyClosingForecast}</Card.Title>
			<div class="flex items-center gap-4 text-xs">
				<span class="flex items-center gap-1.5">
					<ColorMarker class="bg-muted-foreground" />
					<span class="text-muted-foreground">{text.openProgress}</span>
					<span class="font-medium tabular-nums">{money(outcome.openTotals)}</span>
				</span>
				<span class="flex items-center gap-1.5">
					<ColorMarker class="bg-foreground" />
					<span class="text-muted-foreground">{text.won}</span>
					<span class="font-medium tabular-nums">{money(outcome.wonTotals)}</span>
				</span>
			</div>
		</Card.Header>
		<Card.Content class="min-w-0">
			{#if chartData.length > 0}
				<Chart.Container config={chartConfig} class="h-64 w-full" aria-label={text.monthlyClosingForecast}>
					<BarChart
						data={chartData}
						x="index"
						axis="x"
						grid
						rule={false}
						series={chartSeries}
						seriesLayout="group"
						padding={{ left: 6, right: 6, bottom: 18 }}
						props={{
							grid: { class: 'stroke-border/60' },
							xAxis: { ticks: axisLabelIndexes, format: formatAxisLabel },
							bars: { strokeWidth: 0, radius: 2 }
						}}
					>
						{#snippet tooltip()}
							<Chart.Tooltip
								anchor="bottom"
								contained={false}
								indicator="dot"
								labelFormatter={formatTooltipLabel}
								class="w-max min-w-40"
								motion="none"
								x="data"
								y="data"
								yOffset={8}
							>
								{#snippet formatter({ value, name, item })}
									<div class="size-2.5 shrink-0 rounded-[2px]" style={`background-color:${item.color}`}></div>
									<div class="flex flex-1 items-center justify-between gap-5 leading-none">
										<span class="whitespace-nowrap text-muted-foreground">{name}</span>
										<span class="font-mono font-medium tabular-nums text-foreground">
											{formatTooltipValue(value, name)}
										</span>
									</div>
								{/snippet}
							</Chart.Tooltip>
						{/snippet}
					</BarChart>
				</Chart.Container>
			{:else}
				<p class="py-8 text-center text-sm text-muted-foreground">{text.noReportData}</p>
			{/if}
		</Card.Content>
	</Card.Root>

	<section class="grid min-w-0 items-stretch gap-4 lg:grid-cols-2 xl:grid-cols-3" data-crm-report-row="secondary">
		<Card.Root class="h-full min-w-0" data-crm-report-card="stage">
			<Card.Header><Card.Title class="text-base">{text.stageReport}</Card.Title></Card.Header>
			<Card.Content class="grid gap-2.5">
				{#each stageBars as row (row.stage)}
					{@const StageIcon = dealStageIcon(row.stage)}
					{@const isOutcome = row.outcome === 'won' || row.outcome === 'lost'}
					<div class={cn('grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 text-sm sm:grid-cols-[7.5rem_minmax(0,1fr)_minmax(5.5rem,auto)]', isOutcome && 'opacity-60')}>
						<span class="flex min-w-0 items-center gap-1.5">
							<StageIcon class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
							<span class="truncate text-muted-foreground">{row.label}</span>
							<span class="shrink-0 font-medium tabular-nums">{row.count}</span>
						</span>
						<div class="order-3 col-span-2 h-2 overflow-hidden rounded-full bg-muted sm:order-none sm:col-span-1">
							<div class="h-full rounded-full bg-foreground" style={`width: ${(row.count / stageBarMaximum) * 100}%`}></div>
						</div>
						<span class="break-all text-right font-medium tabular-nums">{money(row.totals)}</span>
					</div>
				{:else}
					<p class="py-8 text-center text-sm text-muted-foreground">{text.noReportData}</p>
				{/each}
			</Card.Content>
		</Card.Root>

		<Card.Root class="h-full min-w-0" data-crm-report-card="progress-kind">
			<Card.Header><Card.Title class="text-base">{text.progressKindReport}</Card.Title></Card.Header>
			<Card.Content class="grid gap-2.5">
				{#each pipelineBars as row (row.pipeline)}
					<div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 text-sm sm:grid-cols-[7.5rem_minmax(0,1fr)_minmax(5.5rem,auto)]">
						<span class="flex min-w-0 items-center gap-1.5">
							<ColorMarker color={row.color} />
							<span class="truncate text-muted-foreground">{row.label}</span>
							<span class="shrink-0 font-medium tabular-nums">{row.count}</span>
						</span>
						<div class="order-3 col-span-2 h-2 overflow-hidden rounded-full bg-muted sm:order-none sm:col-span-1">
							<div
								class={cn('h-full rounded-full', row.color ? '' : 'bg-foreground')}
								style={`width: ${(row.count / pipelineBarMaximum) * 100}%${row.color ? `; background-color: ${row.color}` : ''}`}
							></div>
						</div>
						<span class="break-all text-right font-medium tabular-nums">{money(row.totals)}</span>
					</div>
				{:else}
					<p class="py-8 text-center text-sm text-muted-foreground">{text.noReportData}</p>
				{/each}
			</Card.Content>
		</Card.Root>

		<Card.Root class="h-full min-w-0 lg:col-span-2 xl:col-span-1" data-crm-report-card="quiet">
			<Card.Header><Card.Title class="text-base">{text.quietOrganizations}</Card.Title></Card.Header>
			<Card.Content class="min-w-0 px-0">
				<Table.Root class="table-auto text-left">
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-full pl-6" aria-sort={quietAriaSort('name')}>
								<CRMTableColumnHeader label={text.organizationName} sortKey="name" sort={quietSort} onSort={toggleQuietSort} />
							</Table.Head>
							<Table.Head class="hidden whitespace-nowrap md:table-cell" aria-sort={quietAriaSort('owner')}>
								<CRMTableColumnHeader label={text.internalOwner} sortKey="owner" sort={quietSort} onSort={toggleQuietSort} />
							</Table.Head>
							<Table.Head class="hidden whitespace-nowrap text-right tabular-nums sm:table-cell" aria-sort={quietAriaSort('lastContact')}>
								<CRMTableColumnHeader label={text.lastContact} sortKey="lastContact" sort={quietSort} onSort={toggleQuietSort} />
							</Table.Head>
							<Table.Head class="whitespace-nowrap pr-6 text-right tabular-nums" aria-sort={quietAriaSort('days')}>
								<CRMTableColumnHeader label={text.elapsedSinceContact} sortKey="days" sort={quietSort} onSort={toggleQuietSort} />
							</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each sortedQuiet as account (account.id)}
							<Table.Row
								class="cursor-pointer hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
								tabindex={0}
								aria-label={account.name}
								onclick={() => onOpenOrganization(account.id)}
								onkeydown={(event) => handleQuietKeydown(event, account.id)}
							>
								<Table.Cell class="w-full whitespace-nowrap pl-6 font-medium">{account.name}</Table.Cell>
								<Table.Cell class="hidden whitespace-nowrap md:table-cell">
									{#if account.ownerName}
										<PersonChip name={displayPersonName(account.ownerName)} email={account.ownerEmail} seed={account.ownerSeed} />
									{:else}
										<span class="text-muted-foreground">{text.none}</span>
									{/if}
								</Table.Cell>
								<Table.Cell class="hidden whitespace-nowrap text-right tabular-nums text-muted-foreground sm:table-cell">
									{formatCRMDate(account.lastContactDate, currentLocale.value)}
								</Table.Cell>
								<Table.Cell
									class={cn(
										'whitespace-nowrap pr-6 text-right tabular-nums',
										account.daysSinceContact > overdueContactDays ? 'text-destructive' : 'text-muted-foreground'
									)}
								>
									{text.daysAgo.replace('{days}', String(account.daysSinceContact))}
								</Table.Cell>
							</Table.Row>
						{:else}
							<Table.Row><Table.Cell colspan={4} class="py-10 text-center text-sm text-muted-foreground">{text.noOrganizations}</Table.Cell></Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</Card.Content>
		</Card.Root>
	</section>

	<Card.Root class="min-w-0" data-crm-report-card="owner">
		<Card.Header><Card.Title class="text-base">{text.ownerReport}</Card.Title></Card.Header>
		<Card.Content class="min-w-0 px-0">
			<Table.Root class="table-auto text-left">
				<Table.Header>
					<Table.Row>
						<Table.Head class="w-full pl-6" aria-sort={ariaSort('name')}>
							<CRMTableColumnHeader label={text.owner} sortKey="name" sort={ownerSort} onSort={toggleOwnerSort} />
						</Table.Head>
						<Table.Head class="hidden whitespace-nowrap text-right tabular-nums sm:table-cell" aria-sort={ariaSort('organizationCount')}>
							<CRMTableColumnHeader label={text.relationships} sortKey="organizationCount" sort={ownerSort} onSort={toggleOwnerSort} />
						</Table.Head>
						<Table.Head class="whitespace-nowrap text-right tabular-nums" aria-sort={ariaSort('openCount')}>
							<CRMTableColumnHeader label={text.openProgress} sortKey="openCount" sort={ownerSort} onSort={toggleOwnerSort} />
						</Table.Head>
						<Table.Head class="whitespace-nowrap text-right tabular-nums" aria-sort={ariaSort('openAmount')}>
							<CRMTableColumnHeader label={text.openDealAmount} sortKey="openAmount" sort={ownerSort} onSort={toggleOwnerSort} />
						</Table.Head>
						<Table.Head class="hidden whitespace-nowrap text-right tabular-nums md:table-cell" aria-sort={ariaSort('wonAmount')}>
							<CRMTableColumnHeader label={text.wonValue} sortKey="wonAmount" sort={ownerSort} onSort={toggleOwnerSort} />
						</Table.Head>
						<Table.Head class="hidden whitespace-nowrap pr-6 text-right tabular-nums md:table-cell" aria-sort={ariaSort('missingActionCount')}>
							<CRMTableColumnHeader label={text.missingActions} sortKey="missingActionCount" sort={ownerSort} onSort={toggleOwnerSort} />
						</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each sortedOwners as owner (owner.name)}
						<Table.Row>
							<Table.Cell class="w-full whitespace-nowrap pl-6 font-medium">
								<PersonChip name={displayPersonName(owner.name)} email={owner.email} seed={owner.seed} />
							</Table.Cell>
							<Table.Cell class="hidden whitespace-nowrap text-right tabular-nums sm:table-cell">{owner.organizationCount}</Table.Cell>
							<Table.Cell class="whitespace-nowrap text-right tabular-nums">{owner.openCount}</Table.Cell>
							<Table.Cell class="whitespace-nowrap text-right tabular-nums">{money(owner.openTotals)}</Table.Cell>
							<Table.Cell class="hidden whitespace-nowrap text-right tabular-nums md:table-cell">{money(owner.wonTotals)}</Table.Cell>
							<Table.Cell class="hidden whitespace-nowrap pr-6 text-right tabular-nums md:table-cell">{owner.missingActionCount}</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row><Table.Cell colspan={6} class="py-10 text-center text-sm text-muted-foreground">{text.noReportData}</Table.Cell></Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
</div>
