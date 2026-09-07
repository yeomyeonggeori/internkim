<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import PersonChip from '$lib/components/person-chip.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { CRMOrganization, CRMNextAction, CRMOpportunity, CRMPipeline, CRMPipelineStage } from './crm-types';
	import { currentCRMDate } from './crm-date';
	import { dealStageIcon } from './crm-status-icons';
	import CRMPipelineBadge from './crm-pipeline-badge.svelte';
	import CRMTableColumnHeader from './crm-table-column-header.svelte';
	import { nextSortState, sortRows, type CRMSortComparators, type CRMSortState } from './crm-table-sort';
	import { crmLabel } from './crm-labels';
	import { daysLabel, findOrganizationByID, findNextActionByID, formatCRMDate, getProgressKind, getStageVariant, opportunityStageLabel } from './crm-view-model';
	import { formatViewMoney } from './crm-money';
	import { crmViewCurrency } from './crm-view-currency.svelte';
		import type { CRMText } from './text';

	type Props = {
		opportunities: CRMOpportunity[];
		organizations: CRMOrganization[];
		pipelines: CRMPipeline[];
		nextActions: CRMNextAction[];
		stages: CRMPipelineStage[];
		text: CRMText;
		onEdit: (opportunityID: string) => void;
	};

	let { opportunities, organizations, pipelines, nextActions, stages, text, onEdit }: Props = $props();

	const comparators: CRMSortComparators<CRMOpportunity> = {
		name: (opportunity) => opportunity.name,
		organization: (opportunity) => findOrganizationByID(organizations, opportunity.organizationID)?.name ?? '',
		pipeline: (opportunity) => pipelineLabelOf(opportunity),
		stage: (opportunity) => stages.find((stage) => stage.stage === opportunity.stage)?.position ?? Number.MAX_SAFE_INTEGER,
		amount: (opportunity) =>
			opportunity.expectedValue === undefined
				? undefined
				: crmViewCurrency.viewAmount(opportunity.expectedValue, opportunity.currency).value,
		targetDate: (opportunity) => opportunity.targetDate,
		owner: (opportunity) => opportunity.ownerName,
		staleDays: (opportunity) => opportunity.staleDays
	};

	const pageSize = 10;
	const attentionThresholdDays = 14;
	let pageIndex = $state(0);
	let sort = $state<CRMSortState | null>(null);
	let sortedOpportunities = $derived(sortRows(opportunities, sort, comparators));
	let pageCount = $derived(Math.max(1, Math.ceil(sortedOpportunities.length / pageSize)));
	let visibleOpportunities = $derived(sortedOpportunities.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

	function toggleSort(key: string): void {
		sort = nextSortState(sort, key);
		pageIndex = 0;
	}

	function ariaSort(key: string): 'ascending' | 'descending' | 'none' {
		return sort?.key === key ? sort.direction : 'none';
	}

	function elapsedDaysLabel(opportunity: CRMOpportunity): string {
		const elapsedDays = daysLabel(opportunity.staleDays, text);
		if (opportunity.staleDays < attentionThresholdDays) return elapsedDays;
		return `${elapsedDays} · ${text.needsAttention}`;
	}

	function previousPage(): void {
		pageIndex = Math.max(0, pageIndex - 1);
	}

	function nextPage(): void {
		pageIndex = Math.min(pageCount - 1, pageIndex + 1);
	}

	function pipelineLabelOf(opportunity: CRMOpportunity): string {
		const kind = getProgressKind(opportunity, findOrganizationByID(organizations, opportunity.organizationID));
		return pipelines.find((pipeline) => pipeline.pipeline === kind)?.label ?? crmLabel(text.progressKinds, kind);
	}

	function isDeadlineMissed(opportunity: CRMOpportunity): boolean {
		const outcome = stages.find((stage) => stage.stage === opportunity.stage)?.outcome;
		if (outcome === 'won' || outcome === 'lost') return false;
		return opportunity.targetDate !== '' && opportunity.targetDate < currentCRMDate();
	}

	function handleRowKeydown(event: KeyboardEvent, opportunityID: string): void {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		onEdit(opportunityID);
	}

	$effect(() => {
		if (pageIndex >= pageCount) pageIndex = pageCount - 1;
		if (pageIndex < 0) pageIndex = 0;
	});
</script>

<div class="min-w-0 max-w-full overflow-hidden rounded-lg border bg-card shadow-sm">
	<div class="min-w-0">
		<Table.Root class="table-auto">
			<Table.Header class="bg-muted/50 text-left">
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="w-full pl-4" aria-sort={ariaSort('name')}><CRMTableColumnHeader label={text.opportunity} sortKey="name" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap sm:table-cell" aria-sort={ariaSort('organization')}><CRMTableColumnHeader label={text.organizationName} sortKey="organization" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap lg:table-cell" aria-sort={ariaSort('pipeline')}><CRMTableColumnHeader label={text.progressKind} sortKey="pipeline" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="whitespace-nowrap" aria-sort={ariaSort('stage')}><CRMTableColumnHeader label={text.stage} sortKey="stage" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="whitespace-nowrap pr-6 text-right tabular-nums md:pr-0" aria-sort={ariaSort('amount')}><CRMTableColumnHeader label={text.expectedValue} sortKey="amount" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap pr-6 text-right tabular-nums md:table-cell lg:pr-0" aria-sort={ariaSort('targetDate')}><CRMTableColumnHeader label={text.targetDate} sortKey="targetDate" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap pr-6 xl:table-cell xl:pr-0">{text.nextAction}</Table.Head>
					<Table.Head class="hidden whitespace-nowrap pr-6 lg:table-cell xl:pr-0" aria-sort={ariaSort('owner')}><CRMTableColumnHeader label={text.progressOwner} sortKey="owner" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap pr-6 text-right tabular-nums xl:table-cell" title={text.stageElapsedDescription} aria-sort={ariaSort('staleDays')}><CRMTableColumnHeader label={text.staleDays} sortKey="staleDays" {sort} onSort={toggleSort} /></Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body class="text-left">
				{#each visibleOpportunities as opportunity (opportunity.id)}
					{@const organization = findOrganizationByID(organizations, opportunity.organizationID)}
					{@const action = findNextActionByID(nextActions, opportunity.nextActionID)}
					<Table.Row
						class="cursor-pointer align-top hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
						tabindex={0}
						aria-label={`${text.editOpportunity} · ${opportunity.name}`}
						onclick={() => onEdit(opportunity.id)}
						onkeydown={(event) => handleRowKeydown(event, opportunity.id)}
					>
						<Table.Cell class="w-full whitespace-normal pl-4 font-medium">
							<div class="flex min-w-0 items-center gap-2">
								<p class="truncate">{opportunity.name}</p>
								{#if opportunity.calendarEventID}
									<Badge variant="outline" class="shrink-0">{text.calendarRegistered}</Badge>
								{:else if opportunity.calendarRegistrationState === 'failed'}
									<Badge variant="destructive" class="shrink-0">{text.calendarCreateError}</Badge>
								{/if}
							</div>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap text-muted-foreground sm:table-cell">
							<p class="truncate">{organization?.name ?? text.none}</p>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap lg:table-cell">
							<CRMPipelineBadge {opportunity} {organization} {pipelines} {text} />
						</Table.Cell>
						<Table.Cell class="whitespace-nowrap">
							{@const StageIcon = dealStageIcon(opportunity.stage)}<Badge variant={getStageVariant(opportunity.stage)}><StageIcon data-icon="inline-start" aria-hidden="true" />{opportunityStageLabel(stages, opportunity.stage, text)}</Badge>
						</Table.Cell>
						<Table.Cell class="whitespace-nowrap pr-6 text-right font-medium tabular-nums md:pr-0">{opportunity.expectedValue === undefined ? text.noValue : formatViewMoney(crmViewCurrency.viewAmount(opportunity.expectedValue, opportunity.currency), text.noValue, currentLocale.value)}</Table.Cell>
						<Table.Cell class={`hidden whitespace-nowrap pr-6 text-right tabular-nums md:table-cell lg:pr-0 ${isDeadlineMissed(opportunity) ? 'text-destructive' : 'text-muted-foreground'}`}>
							{opportunity.targetDate ? formatCRMDate(opportunity.targetDate, currentLocale.value) : text.none}
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap pr-6 text-muted-foreground xl:table-cell xl:pr-0">
							<p class="truncate">{action?.title ?? text.none}</p>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap pr-6 lg:table-cell xl:pr-0">
							{#if opportunity.ownerName}
								<PersonChip
									name={displayPersonName(opportunity.ownerName)}
									email={opportunity.ownerEmail}
									seed={opportunity.ownerPersonID || opportunity.ownerEmail || opportunity.ownerName}
								/>
							{:else}
								<p class="text-muted-foreground">{text.none}</p>
							{/if}
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap pr-6 text-right tabular-nums text-muted-foreground xl:table-cell">{elapsedDaysLabel(opportunity)}</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent"><Table.Cell colspan={9} class="py-10 text-center text-sm text-muted-foreground">{text.noProgress}</Table.Cell></Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
			totalItems={sortedOpportunities.length}
			{pageIndex}
			{pageSize}
			{pageCount}
			canPreviousPage={pageIndex > 0}
			canNextPage={pageIndex < pageCount - 1}
			{previousPage}
			{nextPage}
			summary={text.paginationSummary}
			previousLabel={text.paginationPrevious}
			nextLabel={text.paginationNext}
		/>
	</div>
</div>
