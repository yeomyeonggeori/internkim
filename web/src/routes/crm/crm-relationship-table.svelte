<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { CRMAccount, CRMContact } from './crm-types';
	import { findPrimaryContactLabel, formatMoneyTotals, getStatusVariant } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		accounts: CRMAccount[];
		contacts: CRMContact[];
		text: CRMText;
		openAccount: (accountID: string) => void;
	};

	let { accounts, contacts, text, openAccount }: Props = $props();

	const pageSize = 10;
	let pageIndex = $state(0);
	let pageCount = $derived(Math.max(1, Math.ceil(accounts.length / pageSize)));
	let visibleAccounts = $derived(accounts.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

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
		<Table.Root class="table-fixed text-left">
			<Table.Header class="bg-muted/50">
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="w-[65%] pl-4 sm:w-[45%] md:w-[40%] lg:w-[30%] xl:w-[26%]">{text.accountName}</Table.Head>
					<Table.Head class="hidden w-[15%] md:table-cell lg:w-[12%] xl:w-[14%]">{text.type}</Table.Head>
					<Table.Head class="w-[35%] sm:w-[20%] md:w-[15%] lg:w-[12%] xl:w-[10%]">{text.status}</Table.Head>
					<Table.Head class="hidden w-[35%] sm:table-cell md:w-[30%] lg:w-[22%] xl:w-[13%]">{text.internalOwner}</Table.Head>
					<Table.Head class="hidden w-[17%] xl:table-cell">{text.externalContact}</Table.Head>
					<Table.Head class="hidden w-[16%] lg:table-cell xl:w-[12%]">{text.expectedValue}</Table.Head>
					<Table.Head class="hidden w-[8%] pr-6 lg:table-cell">{text.openOpportunities}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each visibleAccounts as account (account.id)}
					<Table.Row class="cursor-pointer align-top hover:bg-muted/40" tabindex={0} onclick={() => openAccount(account.id)}>
						<Table.Cell class="whitespace-normal pl-4">
							<div class="min-w-0">
								<p class="truncate font-medium">{account.name}</p>
								<p class="mt-1 hidden line-clamp-2 text-xs leading-5 text-muted-foreground sm:block">{account.description}</p>
							</div>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal md:table-cell">
							<div class="flex flex-wrap gap-1.5">
								{#each account.types as accountType (accountType)}
									<Badge variant="outline">{text.accountTypes[accountType]}</Badge>
								{/each}
							</div>
						</Table.Cell>
						<Table.Cell class="whitespace-normal">
							<Badge variant={getStatusVariant(account.status)}>{text.accountStatuses[account.status]}</Badge>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal sm:table-cell">
							<p class="truncate">{account.ownerName}</p>
							<p class="mt-1 text-xs text-muted-foreground">{account.team}</p>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-muted-foreground xl:table-cell">
							<p class="truncate">{findPrimaryContactLabel(account.id, contacts) || text.none}</p>
						</Table.Cell>
						<Table.Cell class="hidden font-medium lg:table-cell">
							{formatMoneyTotals(account.expectedValues, text.noValue)}
						</Table.Cell>
						<Table.Cell class="hidden pr-6 font-medium lg:table-cell">
							{account.openOpportunityCount}
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent">
						<Table.Cell colspan={7} class="py-10 text-center text-sm text-muted-foreground">{text.noAccounts}</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
			totalItems={accounts.length}
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
