<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import PersonChip from '$lib/components/person-chip.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import * as Empty from '$lib/components/ui/empty';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { CRMDefinition } from './crm-api-types';
	import { crmDefinitionLabel, crmLabel } from './crm-labels';
	import { accountStatusIcon } from './crm-status-icons';
	import CRMTableColumnHeader from './crm-table-column-header.svelte';
	import { nextSortState, sortRows, type CRMSortComparators, type CRMSortState } from './crm-table-sort';
	import type { CRMOrganization, CRMContact } from './crm-types';
	import { currentCRMDate } from './crm-date';
	import { accountStatusRank, findOrganizationContactLabel, formatCRMDate, getStatusVariant } from './crm-view-model';
	import { collapsedViewMoneyTotal, formatViewMoneyTotals } from './crm-money';
	import { crmViewCurrency } from './crm-view-currency.svelte';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import type { CRMText } from './text';

	type Props = {
		organizations: CRMOrganization[];
		contacts: CRMContact[];
		organizationTypeDefinitions: CRMDefinition[];
		currencyCatalogue: CurrencyCatalogue;
		text: CRMText;
		emptyLabel?: string;
		openOrganization: (organizationID: string) => void;
	};

	let { organizations, contacts, organizationTypeDefinitions, currencyCatalogue, text, emptyLabel, openOrganization }: Props = $props();

	function openAmountOf(organization: CRMOrganization): number | undefined {
		const collapsed = collapsedViewMoneyTotal(organization.expectedValues, crmViewCurrency);
		if (collapsed !== undefined) return collapsed;
		const amounts = Object.values(organization.expectedValues);
		if (amounts.length === 0) return undefined;
		return amounts.reduce<number>((total, amount) => total + (amount ?? 0), 0);
	}

	function isOverdue(date: string): boolean {
		return date !== '' && date < currentCRMDate();
	}

	const comparators: CRMSortComparators<CRMOrganization> = {
		name: (organization) => organization.name,
		status: (organization) => accountStatusRank(organization.status),
		owner: (organization) => organization.ownerName,
		lastContact: (organization) => organization.lastContactDate,
		nextAction: (organization) => organization.nextActionDate,
		openValue: openAmountOf
	};

	const pageSize = 10;
	let pageIndex = $state(0);
	let sort = $state<CRMSortState | null>({ key: 'lastContact', direction: 'descending' });
	let sortedOrganizations = $derived(
		sortRows(organizations, sort, comparators, {
			read: (organization: CRMOrganization) => accountStatusRank(organization.status),
			direction: 'descending'
		})
	);
	let pageCount = $derived(Math.max(1, Math.ceil(sortedOrganizations.length / pageSize)));
	let visibleOrganizations = $derived(sortedOrganizations.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

	function toggleSort(key: string): void {
		sort = nextSortState(sort, key);
		pageIndex = 0;
	}

	function ariaSort(key: string): 'ascending' | 'descending' | 'none' {
		return sort?.key === key ? sort.direction : 'none';
	}

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
		<Table.Root class="table-auto">
			<Table.Header class="bg-muted/50 text-left">
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="w-full pl-4" aria-sort={ariaSort('name')}><CRMTableColumnHeader label={text.organizationName} sortKey="name" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap md:table-cell">{text.type}</Table.Head>
					<Table.Head class="whitespace-nowrap" aria-sort={ariaSort('status')}><CRMTableColumnHeader label={text.status} sortKey="status" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap sm:table-cell" aria-sort={ariaSort('owner')}><CRMTableColumnHeader label={text.internalOwner} sortKey="owner" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap xl:table-cell">{text.externalContact}</Table.Head>
					<Table.Head class="hidden whitespace-nowrap text-right tabular-nums lg:table-cell" aria-sort={ariaSort('lastContact')}><CRMTableColumnHeader label={text.lastContact} sortKey="lastContact" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap text-right tabular-nums xl:table-cell" aria-sort={ariaSort('nextAction')}><CRMTableColumnHeader label={text.nextAction} sortKey="nextAction" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="whitespace-nowrap pr-6 text-right tabular-nums" aria-sort={ariaSort('openValue')}><CRMTableColumnHeader label={text.kpiOpenValue} sortKey="openValue" {sort} onSort={toggleSort} /></Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body class="text-left">
				{#each visibleOrganizations as organization (organization.id)}
					<Table.Row class="cursor-pointer align-top hover:bg-muted/40" tabindex={0} onclick={() => openOrganization(organization.id)}>
						<Table.Cell class="w-full whitespace-normal pl-4">
							<div class="min-w-0">
								<p class="truncate font-medium">{organization.name}</p>
								<p class="mt-1 hidden line-clamp-2 text-xs leading-5 text-muted-foreground sm:block">{organization.description}</p>
							</div>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap pl-0 md:table-cell">
							<div class="flex gap-1.5">
								{#each organization.types as organizationType, index (organizationType)}
									<Badge variant="outline" data-crm-leading-pill-text={index === 0 ? '' : undefined}>{crmDefinitionLabel(organizationTypeDefinitions, text.organizationTypes, organizationType)}</Badge>
								{/each}
							</div>
						</Table.Cell>
						<Table.Cell class="whitespace-nowrap">
							{@const StatusIcon = accountStatusIcon(organization.status)}<Badge variant={getStatusVariant(organization.status)} data-crm-leading-pill><StatusIcon data-icon="inline-start" aria-hidden="true" />{text.organizationStatuses[organization.status]}</Badge>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap sm:table-cell">
							{#if organization.ownerName}
								<PersonChip
									name={displayPersonName(organization.ownerName)}
									email={organization.ownerEmail}
									seed={organization.ownerPersonID || organization.ownerEmail || organization.ownerName}
								/>
							{:else}
								<p class="text-muted-foreground">{text.none}</p>
							{/if}
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap text-muted-foreground xl:table-cell">
							<p class="truncate">{findOrganizationContactLabel(organization.id, contacts) || text.none}</p>
						</Table.Cell>
						<Table.Cell class="whitespace-nowrap hidden text-right tabular-nums text-muted-foreground lg:table-cell">
							{organization.lastContactDate ? formatCRMDate(organization.lastContactDate, currentLocale.value) : text.none}
						</Table.Cell>
						<Table.Cell class={`hidden whitespace-nowrap text-right tabular-nums xl:table-cell ${isOverdue(organization.nextActionDate) ? 'text-destructive' : 'text-muted-foreground'}`}>
							{organization.nextActionDate ? formatCRMDate(organization.nextActionDate, currentLocale.value) : text.none}
						</Table.Cell>
						<Table.Cell class="whitespace-nowrap pr-6 text-right font-medium tabular-nums">
							{formatViewMoneyTotals(organization.expectedValues, currencyCatalogue, crmViewCurrency, text.noValue, currentLocale.value)}
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent">
						<Table.Cell colspan={8} class="whitespace-normal p-0"><Empty.Root><Empty.Header><Empty.Title>{emptyLabel ?? text.organizationsEmpty}</Empty.Title></Empty.Header></Empty.Root></Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
 onPageChange={(page) => (pageIndex = page)}
			totalItems={sortedOrganizations.length}
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
