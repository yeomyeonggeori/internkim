<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import CRMCurrencyComparisonChart from './crm-currency-comparison-chart.svelte';
	import { buildCRMReportPeriodBounds } from './crm-date';
	import { crmLabel } from './crm-labels';
	import type { CRMOrganization, CRMMoneyTotals, CRMNextAction, CRMOpportunity, CRMPipelineStage, CRMProgressKind } from './crm-types';
	import { formatMoneyTotals, sumOpportunityMoney } from './crm-money';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import {
		findOrganizationByID,
		formatCRMDate,
		getProgressKind,
		opportunityStageLabel
	} from './crm-view-model';
	import type { CRMText } from './text';

	type ReportPeriod = 'quarter' | 'next_90_days' | 'all';
	type MetricRow = { key: string; label: string; count: number; values: CRMMoneyTotals };
	type OwnerRow = {
		name: string;
		organizationCount: number;
		openCount: number;
		openValues: CRMMoneyTotals;
		missingActionCount: number;
	};

	type Props = {
		organizations: CRMOrganization[];
		opportunities: CRMOpportunity[];
		nextActions: CRMNextAction[];
		stages: CRMPipelineStage[];
		currencyCatalogue: CurrencyCatalogue;
		text: CRMText;
		onOpenOrganization: (organizationID: string) => void;
	};

	let { organizations, opportunities, nextActions, stages, currencyCatalogue, text, onOpenOrganization }: Props = $props();
	const { today, next90DaysEnd, quarterStart, quarterEnd } = buildCRMReportPeriodBounds();
	let period = $state<ReportPeriod>('all');

	let periodOpportunities = $derived(
		opportunities.filter((opportunity) => {
			if (period === 'all') return true;
			if (period === 'next_90_days') {
				return opportunity.targetDate >= today && opportunity.targetDate <= next90DaysEnd;
			}
			return opportunity.targetDate >= quarterStart && opportunity.targetDate <= quarterEnd;
		})
	);
	let openOpportunities = $derived(periodOpportunities.filter((opportunity) => opportunityOutcome(opportunity) === 'open'));
	let openValues = $derived(sumOpportunityMoney(openOpportunities));
	let wonValues = $derived(sumOpportunityMoney(periodOpportunities.filter((opportunity) => opportunityOutcome(opportunity) === 'won')));
	let monthlyRows = $derived.by(() => {
		const months = [...new Set(periodOpportunities.map((opportunity) => opportunity.targetDate.slice(0, 7)))];
		return months.sort().map((month) => {
			const matching = periodOpportunities.filter((opportunity) => opportunity.targetDate.startsWith(month));
			return {
				month,
				openValues: sumOpportunityMoney(matching.filter((opportunity) => opportunityOutcome(opportunity) === 'open')),
				wonValues: sumOpportunityMoney(matching.filter((opportunity) => opportunityOutcome(opportunity) === 'won'))
			};
		});
	});
	let stageRows = $derived(
		[...new Set(periodOpportunities.map((opportunity) => opportunity.stage))].map((stage) => {
			const matching = periodOpportunities.filter((opportunity) => opportunity.stage === stage);
			return {
				key: stage,
				label: opportunityStageLabel(stages, stage, text),
				count: matching.length,
				values: sumOpportunityMoney(matching)
			};
		})
	);
	let progressKindRows = $derived.by(() => {
		const kinds: CRMProgressKind[] = ['sales', 'fundraising', 'investment', 'sponsorship', 'partnership', 'procurement'];
		return kinds.map((kind) => {
			const matching = periodOpportunities.filter(
				(opportunity) =>
					getProgressKind(opportunity, findOrganizationByID(organizations, opportunity.organizationID)) === kind
			);
			return {
				key: kind,
				label: crmLabel(text.progressKinds, kind),
				count: matching.length,
				values: sumOpportunityMoney(matching)
			};
		});
	});
	let ownerRows = $derived.by(() => {
		const ownerNames = [...new Set([...organizations.map((organization) => organization.ownerName), ...periodOpportunities.map((opportunity) => opportunity.ownerName)])];
		return ownerNames
			.map<OwnerRow>((name) => {
				const ownerOpportunities = openOpportunities.filter((opportunity) => opportunity.ownerName === name);
				return {
					name,
					organizationCount: organizations.filter((organization) => organization.ownerName === name).length,
					openCount: ownerOpportunities.length,
					openValues: sumOpportunityMoney(ownerOpportunities),
					missingActionCount: ownerOpportunities.filter(
						(opportunity) =>
							!opportunity.nextActionID ||
							!nextActions.some((action) => action.id === opportunity.nextActionID && action.status !== 'done')
					).length
				};
			})
			.sort((left, right) => right.openCount - left.openCount);
	});
	let quietOrganizations = $derived(
		[...organizations]
			.sort((left, right) => left.lastContactDate.localeCompare(right.lastContactDate))
			.slice(0, 5)
	);

	function maximumValue(rows: MetricRow[]): number {
		return Math.max(1, ...rows.map((row) => row.count));
	}

	function opportunityOutcome(opportunity: CRMOpportunity): CRMPipelineStage['outcome'] | undefined {
		return stages.find(
			(stage) => stage.pipeline === (opportunity.pipeline ?? opportunity.kind) && stage.stage === opportunity.stage
		)?.outcome;
	}
</script>

<div class="grid min-w-0 gap-4" data-crm-report>
	<div class="flex flex-col gap-3 rounded-md border bg-card p-3 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<p class="text-sm font-semibold">{text.reportPeriod}</p>
			<p class="text-xs text-muted-foreground">{text.reportPeriodDescription}</p>
		</div>
		<div class="flex flex-col gap-2 sm:flex-row">
			<Select.Root type="single" value={period} onValueChange={(value) => (period = value as ReportPeriod)}>
				<Select.Trigger class="w-full sm:w-44" aria-label={text.reportPeriod}>
					{period === 'quarter' ? text.reportPeriodQuarter : period === 'next_90_days' ? text.reportPeriodNext90Days : text.reportPeriodAll}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="quarter" label={text.reportPeriodQuarter}>{text.reportPeriodQuarter}</Select.Item>
					<Select.Item value="next_90_days" label={text.reportPeriodNext90Days}>{text.reportPeriodNext90Days}</Select.Item>
					<Select.Item value="all" label={text.reportPeriodAll}>{text.reportPeriodAll}</Select.Item>
				</Select.Content>
			</Select.Root>
		</div>
	</div>

	<section class="grid min-w-0 items-stretch gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(20rem,2fr)]" aria-label={text.reportSummary} data-crm-report-row="primary">
		<CRMCurrencyComparisonChart expectedTotals={openValues} wonTotals={wonValues} {currencyCatalogue} {text} />
		<Card.Root class="h-full min-w-0" data-crm-report-card="monthly">
			<Card.Header><Card.Title class="text-base">{text.monthlyClosingForecast}</Card.Title><Card.Description>{text.monthlyClosingForecastDescription}</Card.Description></Card.Header>
			<Card.Content class="grid gap-4">
				{#each monthlyRows as row (row.month)}
					<div class="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-3">
						<span class="text-sm font-medium">{row.month.replace('-', '. ')}</span>
						<div class="grid gap-2">
							<div class="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2 text-xs"><span class="text-muted-foreground">{text.openValue}</span><span class="text-right font-medium">{formatMoneyTotals(row.openValues, currencyCatalogue, text.noValue, currentLocale.value)}</span></div>
							<div class="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2 text-xs"><span class="text-muted-foreground">{text.won}</span><span class="text-right font-medium">{formatMoneyTotals(row.wonValues, currencyCatalogue, text.noValue, currentLocale.value)}</span></div>
						</div>
					</div>
				{:else}
					<p class="py-8 text-center text-sm text-muted-foreground">{text.noReportData}</p>
				{/each}
			</Card.Content>
		</Card.Root>
	</section>

	<section class="grid min-w-0 items-stretch gap-4 lg:grid-cols-2 xl:grid-cols-3" data-crm-report-row="secondary">
		<Card.Root class="h-full min-w-0" data-crm-report-card="progress-kind">
			<Card.Header><Card.Title class="text-base">{text.progressKindReport}</Card.Title><Card.Description>{text.progressKindReportDescription}</Card.Description></Card.Header>
			<Card.Content class="grid gap-3">
				{#each progressKindRows as row (row.key)}
					<div class="grid grid-cols-[5.5rem_minmax(0,1fr)_minmax(7rem,auto)] items-center gap-2 text-sm">
						<span class="text-muted-foreground">{row.label} <strong class="text-foreground">{row.count}</strong></span>
						<div class="h-2 overflow-hidden rounded-full bg-muted"><div class="h-full rounded-full bg-primary/75" style={`width: ${(row.count / maximumValue(progressKindRows)) * 100}%`}></div></div>
						<span class="text-right font-medium">{formatMoneyTotals(row.values, currencyCatalogue, text.noValue, currentLocale.value)}</span>
					</div>
				{/each}
			</Card.Content>
		</Card.Root>

		<Card.Root class="h-full min-w-0" data-crm-report-card="stage">
			<Card.Header><Card.Title class="text-base">{text.stageReport}</Card.Title><Card.Description>{text.stageReportDescription}</Card.Description></Card.Header>
			<Card.Content class="grid gap-3">
				{#each stageRows as row (row.key)}
					<div class="grid grid-cols-[5rem_minmax(0,1fr)_minmax(7rem,auto)] items-center gap-2 text-sm">
						<span class="truncate text-muted-foreground">{row.label} <strong class="text-foreground">{row.count}</strong></span>
						<div class="h-2 overflow-hidden rounded-full bg-muted"><div class="h-full rounded-full bg-foreground" style={`width: ${(row.count / maximumValue(stageRows)) * 100}%`}></div></div>
						<span class="text-right font-medium">{formatMoneyTotals(row.values, currencyCatalogue, text.noValue, currentLocale.value)}</span>
					</div>
				{/each}
			</Card.Content>
		</Card.Root>

		<Card.Root class="h-full min-w-0 lg:col-span-2 xl:col-span-1" data-crm-report-card="quiet">
			<Card.Header><Card.Title class="text-base">{text.quietOrganizations}</Card.Title><Card.Description>{text.quietOrganizationsDescription}</Card.Description></Card.Header>
			<Card.Content class="grid gap-1">
				{#each quietOrganizations as organization (organization.id)}
					<button type="button" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md px-2 py-2 text-left hover:bg-muted" onclick={() => onOpenOrganization(organization.id)}>
						<span class="truncate text-sm font-medium">{organization.name}</span>
						<span class="text-xs text-muted-foreground">{formatCRMDate(organization.lastContactDate, currentLocale.value)}</span>
					</button>
				{/each}
			</Card.Content>
		</Card.Root>
	</section>

	<Card.Root class="min-w-0">
		<Card.Header><Card.Title class="text-base">{text.ownerReport}</Card.Title><Card.Description>{text.ownerReportDescription}</Card.Description></Card.Header>
		<Card.Content class="min-w-0 px-0">
			<Table.Root class="table-fixed text-left">
				<Table.Header><Table.Row><Table.Head class="w-[45%] pl-6 sm:w-[35%] md:w-[25%] lg:w-[20%]">{text.owner}</Table.Head><Table.Head class="hidden w-[15%] sm:table-cell">{text.relationships}</Table.Head><Table.Head class="w-[20%] sm:w-[15%]">{text.openProgress}</Table.Head><Table.Head class="w-[35%] sm:w-[35%] md:w-[25%] lg:w-[30%]">{text.openValue}</Table.Head><Table.Head class="hidden w-[20%] pr-6 md:table-cell">{text.missingActions}</Table.Head></Table.Row></Table.Header>
				<Table.Body>
					{#each ownerRows as owner (owner.name)}
						<Table.Row>
							<Table.Cell class="whitespace-normal pl-6 font-medium"><p class="truncate">{owner.name}</p></Table.Cell>
							<Table.Cell class="hidden sm:table-cell">{owner.organizationCount}</Table.Cell>
							<Table.Cell>{owner.openCount}</Table.Cell>
							<Table.Cell class="whitespace-normal">{formatMoneyTotals(owner.openValues, currencyCatalogue, text.noValue, currentLocale.value)}</Table.Cell>
							<Table.Cell class="hidden pr-6 md:table-cell"><Badge variant={owner.missingActionCount > 0 ? 'secondary' : 'outline'}>{owner.missingActionCount}</Badge></Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
</div>
