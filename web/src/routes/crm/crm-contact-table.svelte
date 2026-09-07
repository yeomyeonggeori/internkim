<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import PersonChip from '$lib/components/person-chip.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import type { UserRecord } from '$lib/organization/types';
	import type { CRMOrganization, CRMContact } from './crm-types';
	import { effectiveContactOwner, findOrganizationByID } from './crm-view-model';
	import CRMTableColumnHeader from './crm-table-column-header.svelte';
	import { nextSortState, sortRows, type CRMSortComparators, type CRMSortState } from './crm-table-sort';
	import type { CRMText } from './text';

	type Props = {
		contacts: CRMContact[];
		organizations: CRMOrganization[];
		people: UserRecord[];
		text: CRMText;
		onEdit: (contactID: string) => void;
	};

	let { contacts, organizations, people, text, onEdit }: Props = $props();
	const comparators: CRMSortComparators<CRMContact> = {
		name: (contact) => contact.name,
		organization: (contact) => findOrganizationByID(organizations, contact.organizationID)?.name ?? '',
		title: (contact) => contact.title,
		owner: (contact) => effectiveContactOwner(contact, organizations, people)?.name ?? ''
	};

	const pageSize = 10;
	let pageIndex = $state(0);
	let sort = $state<CRMSortState | null>(null);
	let sortedContacts = $derived(sortRows(contacts, sort, comparators));
	let pageCount = $derived(Math.max(1, Math.ceil(sortedContacts.length / pageSize)));
	let visibleContacts = $derived(sortedContacts.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

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

	function handleRowKeydown(event: KeyboardEvent, contactID: string): void {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		onEdit(contactID);
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
					<Table.Head class="w-full pl-4" aria-sort={ariaSort('name')}><CRMTableColumnHeader label={text.contactName} sortKey="name" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="whitespace-nowrap pr-6 sm:pr-0" aria-sort={ariaSort('organization')}><CRMTableColumnHeader label={text.organizationName} sortKey="organization" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap md:table-cell" aria-sort={ariaSort('title')}><CRMTableColumnHeader label={text.contactTitle} sortKey="title" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap pr-6 sm:table-cell sm:pr-6 lg:pr-0">{text.email}</Table.Head>
					<Table.Head class="hidden whitespace-nowrap pr-6 lg:table-cell xl:pr-0" aria-sort={ariaSort('owner')}><CRMTableColumnHeader label={text.internalOwner} sortKey="owner" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap pr-6 xl:table-cell">{text.phone}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body class="text-left">
				{#each visibleContacts as contact (contact.id)}
					{@const organization = findOrganizationByID(organizations, contact.organizationID)}
					{@const owner = effectiveContactOwner(contact, organizations, people)}
					<Table.Row
						class="cursor-pointer align-top hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
						tabindex={0}
						aria-label={`${text.editContact} · ${contact.name}`}
						onclick={() => onEdit(contact.id)}
						onkeydown={(event) => handleRowKeydown(event, contact.id)}
					>
						<Table.Cell class="w-full whitespace-normal pl-4">
							<div class="flex flex-wrap items-center gap-2">
								<span class="font-medium">{contact.name}</span>
							</div>
						</Table.Cell>
						<Table.Cell class="whitespace-nowrap pr-6 text-muted-foreground sm:pr-0"><p class="truncate">{organization?.name ?? text.none}</p></Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap md:table-cell">{contact.title || text.none}</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap pr-6 sm:table-cell lg:pr-0"><a class="block truncate text-primary hover:underline" href={`mailto:${contact.email}`} onclick={(event) => event.stopPropagation()} onkeydown={(event) => event.stopPropagation()}>{contact.email}</a></Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap pr-6 lg:table-cell xl:pr-0">
							{#if owner}
								<PersonChip name={displayPersonName(owner.name)} email={owner.email} seed={owner.seed} />
							{:else}
								<span class="text-muted-foreground">{text.none}</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap pr-6 xl:table-cell">{#if contact.phone}<a class="block truncate hover:underline" href={`tel:${contact.phone}`} onclick={(event) => event.stopPropagation()} onkeydown={(event) => event.stopPropagation()}>{contact.phone}</a>{:else}{text.none}{/if}</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent"><Table.Cell colspan={6} class="py-10 text-center text-sm text-muted-foreground">{text.noContacts}</Table.Cell></Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
			totalItems={sortedContacts.length}
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
