<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { CRMAccount, CRMContact } from './crm-types';
	import { findAccountByID } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		contacts: CRMContact[];
		accounts: CRMAccount[];
		text: CRMText;
		onEdit: (contactID: string) => void;
	};

	let { contacts, accounts, text, onEdit }: Props = $props();
	const pageSize = 10;
	let pageIndex = $state(0);
	let pageCount = $derived(Math.max(1, Math.ceil(contacts.length / pageSize)));
	let visibleContacts = $derived(contacts.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

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
		<Table.Root class="table-fixed text-left">
			<Table.Header class="bg-muted/50">
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="w-[45%] pl-4 sm:w-[30%] md:w-[25%] lg:w-[22%] xl:w-[16%]">{text.contactName}</Table.Head>
					<Table.Head class="w-[55%] sm:w-[35%] md:w-[25%] lg:w-[22%] xl:w-[18%]">{text.accountName}</Table.Head>
					<Table.Head class="hidden w-[15%] md:table-cell lg:w-[14%] xl:w-[14%]">{text.contactTitle}</Table.Head>
					<Table.Head class="hidden w-[35%] sm:table-cell md:w-[35%] lg:w-[28%] xl:w-[20%]">{text.email}</Table.Head>
					<Table.Head class="hidden w-[14%] lg:table-cell">{text.phone}</Table.Head>
					<Table.Head class="hidden w-[18%] xl:table-cell">{text.details}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each visibleContacts as contact (contact.id)}
					{@const account = findAccountByID(accounts, contact.accountID)}
					<Table.Row
						class="cursor-pointer align-top hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
						tabindex={0}
						aria-label={`${text.editContact} · ${contact.name}`}
						onclick={() => onEdit(contact.id)}
						onkeydown={(event) => handleRowKeydown(event, contact.id)}
					>
						<Table.Cell class="whitespace-normal pl-4">
							<div class="flex flex-wrap items-center gap-2">
								<span class="font-medium">{contact.name}</span>
								{#if contact.isPrimary}<Badge variant="secondary">{text.primary}</Badge>{/if}
							</div>
						</Table.Cell>
						<Table.Cell class="whitespace-normal text-muted-foreground"><p class="truncate">{account?.name ?? text.none}</p></Table.Cell>
						<Table.Cell class="hidden whitespace-normal md:table-cell">{contact.title || text.none}</Table.Cell>
						<Table.Cell class="hidden whitespace-normal sm:table-cell"><a class="block truncate text-primary hover:underline" href={`mailto:${contact.email}`} onclick={(event) => event.stopPropagation()} onkeydown={(event) => event.stopPropagation()}>{contact.email}</a></Table.Cell>
						<Table.Cell class="hidden whitespace-normal lg:table-cell">{#if contact.phone}<a class="block truncate hover:underline" href={`tel:${contact.phone}`} onclick={(event) => event.stopPropagation()} onkeydown={(event) => event.stopPropagation()}>{contact.phone}</a>{:else}{text.none}{/if}</Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-sm text-muted-foreground xl:table-cell"><p class="line-clamp-2">{contact.note ?? text.none}</p></Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent"><Table.Cell colspan={6} class="py-10 text-center text-sm text-muted-foreground">{text.noContacts}</Table.Cell></Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
			totalItems={contacts.length}
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
