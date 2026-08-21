<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { CRMOrganization, CRMNextAction, CRMOpportunity, CRMPipelineStage } from './crm-types';
	import { daysLabel, findOrganizationByID, findNextActionByID, formatMoney, getStageVariant, opportunityStageLabel } from './crm-view-model';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import type { CRMText } from './text';

	type Props = {
		opportunities: CRMOpportunity[];
		organizations: CRMOrganization[];
		nextActions: CRMNextAction[];
		stages: CRMPipelineStage[];
		currencyCatalogue: CurrencyCatalogue;
		text: CRMText;
		onEdit: (opportunityID: string) => void;
	};

	let { opportunities, organizations, nextActions, stages, currencyCatalogue, text, onEdit }: Props = $props();

	const pageSize = 10;
	const attentionThresholdDays = 14;
	let pageIndex = $state(0);
	let pageCount = $derived(Math.max(1, Math.ceil(opportunities.length / pageSize)));
	let visibleOpportunities = $derived(opportunities.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

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
		<Table.Root class="table-fixed">
			<Table.Header class="bg-muted/50 text-left">
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="w-[65%] sm:w-[40%] md:w-[35%] lg:w-[27%] xl:w-[19%]">{text.opportunity}</Table.Head>
					<Table.Head class="hidden w-[25%] sm:table-cell md:w-[20%] lg:w-[17%] xl:w-[14%]">{text.organizationName}</Table.Head>
					<Table.Head class="hidden w-[9%] xl:table-cell">{text.business}</Table.Head>
					<Table.Head class="w-[35%] text-center sm:w-[20%] md:w-[15%] lg:w-[12%] xl:w-[9%]">{text.stage}</Table.Head>
					<Table.Head class="hidden w-[15%] md:table-cell lg:w-[12%] xl:w-[11%]">{text.progressOwner}</Table.Head>
					<Table.Head class="hidden w-[15%] sm:table-cell lg:w-[14%] xl:w-[12%]">{text.expectedValue}</Table.Head>
					<Table.Head class="hidden w-[18%] lg:table-cell">{text.nextAction}</Table.Head>
					<Table.Head class="hidden w-[8%] xl:table-cell" title={text.stageElapsedDescription}>{text.staleDays}</Table.Head>
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
						<Table.Cell class="whitespace-normal font-medium">
							<div class="flex min-w-0 items-center gap-2">
								<p class="truncate">{opportunity.name}</p>
								{#if opportunity.calendarEventID}
									<Badge variant="outline" class="shrink-0">{text.calendarRegistered}</Badge>
								{:else if opportunity.calendarRegistrationState === 'failed'}
									<Badge variant="destructive" class="shrink-0">{text.calendarCreateError}</Badge>
								{/if}
							</div>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-muted-foreground sm:table-cell">
							<p class="truncate">{organization?.name ?? text.none}</p>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal xl:table-cell"><p class="truncate">{opportunity.business}</p></Table.Cell>
						<Table.Cell class="whitespace-normal text-center">
							<Badge variant={getStageVariant(opportunity.stage)} data-crm-centered-pill>{opportunityStageLabel(stages, opportunity.stage, text)}</Badge>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal md:table-cell">
							<p class="truncate">{opportunity.ownerName}</p>
						</Table.Cell>
						<Table.Cell class="hidden font-medium sm:table-cell">{formatMoney(opportunity.expectedValue, opportunity.currency, currencyCatalogue, text.noValue, currentLocale.value)}</Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-muted-foreground lg:table-cell">
							<p class="truncate">{action?.title ?? text.none}</p>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-muted-foreground xl:table-cell">{elapsedDaysLabel(opportunity)}</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent"><Table.Cell colspan={8} class="py-10 text-center text-sm text-muted-foreground">{text.noProgress}</Table.Cell></Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
			totalItems={opportunities.length}
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
