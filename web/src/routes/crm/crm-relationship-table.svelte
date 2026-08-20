<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { CRMDefinition } from './crm-api-types';
	import { crmDefinitionLabel, crmLabel } from './crm-labels';
	import type { CRMOrganization, CRMContact } from './crm-types';
	import { crmInterimCurrencyCatalogue, findOrganizationContactLabel, formatMoneyTotals, getStatusVariant } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		organizations: CRMOrganization[];
		contacts: CRMContact[];
		organizationTypeDefinitions: CRMDefinition[];
		text: CRMText;
		openOrganization: (organizationID: string) => void;
	};

	let { organizations, contacts, organizationTypeDefinitions, text, openOrganization }: Props = $props();

	const pageSize = 10;
	let pageIndex = $state(0);
	let pageCount = $derived(Math.max(1, Math.ceil(organizations.length / pageSize)));
	let visibleOrganizations = $derived(organizations.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

	function previousPage(): void {
		pageIndex = Math.max(0, pageIndex - 1);
	}

	function nextPage(): void {
		pageIndex = Math.min(pageCount - 1, pageIndex + 1);
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
					<Table.Head class="w-[65%] pl-4 sm:w-[45%] md:w-[40%] lg:w-[30%] xl:w-[26%]">{text.organizationName}</Table.Head>
					<Table.Head class="hidden w-[15%] md:table-cell lg:w-[12%] xl:w-[14%]">{text.type}</Table.Head>
					<Table.Head class="w-[35%] sm:w-[20%] md:w-[15%] lg:w-[12%] xl:w-[10%]">{text.status}</Table.Head>
					<Table.Head class="hidden w-[35%] sm:table-cell md:w-[30%] lg:w-[22%] xl:w-[13%]">{text.internalOwner}</Table.Head>
					<Table.Head class="hidden w-[17%] xl:table-cell">{text.externalContact}</Table.Head>
					<Table.Head class="hidden w-[16%] lg:table-cell xl:w-[12%]">{text.expectedValue}</Table.Head>
					<Table.Head class="hidden w-[8%] pr-6 lg:table-cell">{text.openOpportunities}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body class="text-left">
				{#each visibleOrganizations as organization (organization.id)}
					<Table.Row class="cursor-pointer align-top hover:bg-muted/40" tabindex={0} onclick={() => openOrganization(organization.id)}>
						<Table.Cell class="whitespace-normal pl-4">
							<div class="min-w-0">
								<p class="truncate font-medium">{organization.name}</p>
								<p class="mt-1 hidden line-clamp-2 text-xs leading-5 text-muted-foreground sm:block">{organization.description}</p>
							</div>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal pl-0 md:table-cell">
							<div class="flex flex-wrap gap-1.5">
								{#each organization.types as organizationType, index (organizationType)}
									<Badge variant="outline" data-crm-leading-pill-text={index === 0 ? '' : undefined}>{crmDefinitionLabel(organizationTypeDefinitions, text.organizationTypes, organizationType)}</Badge>
								{/each}
							</div>
						</Table.Cell>
						<Table.Cell class="whitespace-normal">
							<Badge variant={getStatusVariant(organization.status)} data-crm-leading-pill>{text.organizationStatuses[organization.status]}</Badge>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal sm:table-cell">
							<p class="truncate">{organization.ownerName}</p>
							<p class="mt-1 text-xs text-muted-foreground">{organization.team}</p>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-muted-foreground xl:table-cell">
							<p class="truncate">{findOrganizationContactLabel(organization.id, contacts) || text.none}</p>
						</Table.Cell>
						<Table.Cell class="hidden font-medium lg:table-cell">
							{formatMoneyTotals(organization.expectedValues, crmInterimCurrencyCatalogue, text.noValue)}
						</Table.Cell>
						<Table.Cell class="hidden pr-6 font-medium lg:table-cell">
							{organization.openOpportunityCount}
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent">
						<Table.Cell colspan={7} class="py-10 text-center text-sm text-muted-foreground">{text.noOrganizations}</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
			totalItems={organizations.length}
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
