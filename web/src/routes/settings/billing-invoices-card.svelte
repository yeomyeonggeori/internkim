<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import { companyInvoices } from '$lib/company/credits';
	import type { CompanyInvoice } from '$lib/company/invoice-contract';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { billingText } from './billing-text';
	import { invoiceAmount, invoiceDay } from './billing-format';

	let { showsWhenEmpty }: { showsWhenEmpty: boolean } = $props();

	const text = createPageText(billingText);

	let invoices = $state<CompanyInvoice[] | null>(null);
	let hasFailed = $state(false);

	const statusLabel = $derived<Record<CompanyInvoice['status'], string>>({
		paid: text.invoicePaid,
		partially_refunded: text.invoicePartiallyRefunded,
		refunded: text.invoiceRefunded,
		open: text.invoiceOpen,
		void: text.invoiceVoid
	});

	onMount(async () => {
		try {
			invoices = await companyInvoices();
		} catch {
			hasFailed = true;
		}
	});
</script>

{#if showsWhenEmpty || (invoices !== null && invoices.length > 0)}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.invoices}</Card.Title>
			<Card.Description>{text.invoicesDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if hasFailed}
				<p class="text-sm text-muted-foreground">{text.invoicesLoadFailed}</p>
			{:else if invoices === null}
				<div role="status" aria-label={text.invoices} aria-busy="true"><Skeleton class="h-20 w-full" /></div>
			{:else if invoices.length === 0}
				<p class="text-sm text-muted-foreground">{text.invoicesEmpty}</p>
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{text.invoiceDate}</Table.Head>
							<Table.Head class="max-sm:hidden">{text.invoiceDescription}</Table.Head>
							<Table.Head class="text-right">{text.invoiceAmount}</Table.Head>
							<Table.Head>{text.invoiceStatus}</Table.Head>
							<Table.Head><span class="sr-only">{text.invoiceReceipt}</span></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each invoices as invoice (invoice.id)}
							<Table.Row>
								<Table.Cell class="tabular-nums">{invoiceDay(invoice.issuedAt, currentLocale.value)}</Table.Cell>
								<Table.Cell class="max-w-56 truncate max-sm:hidden">{invoice.description}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{invoiceAmount(invoice, currentLocale.value)}</Table.Cell>
								<Table.Cell>
									<Badge variant={invoice.status === 'paid' ? 'secondary' : 'outline'}>{statusLabel[invoice.status]}</Badge>
								</Table.Cell>
								<Table.Cell class="text-right">
									{#if invoice.receiptURL}
										<a href={invoice.receiptURL} target="_blank" rel="noopener noreferrer" class="text-sm underline">{text.invoiceReceipt}</a>
									{/if}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		</Card.Content>
	</Card.Root>
{/if}
