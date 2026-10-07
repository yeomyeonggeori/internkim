<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import * as Tabs from '$lib/components/ui/tabs';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import { isViewCurrencyAvailable } from './crm-view-currency.svelte';
	import type { CRMText } from './text';
	let { text, tabs }: { text: CRMText; tabs: { value: string; label: string }[] } = $props();
</script>

{#snippet sortableHeading(label: string)}
	<Button variant="ghost" size="sm" disabled class="-ml-2 h-8 gap-1.5 px-2 font-medium">{label}<ArrowUpDownIcon class="size-3.5 shrink-0 opacity-60" /></Button>
{/snippet}

<div role="status" aria-label={text.loading} aria-busy="true" class="grid min-w-0 gap-2 sm:gap-4" data-testid="crm-loading-skeleton">
	<div class="min-w-0">
		<button disabled class="flex min-h-11 w-full items-center gap-1 text-sm font-medium sm:hidden">{text.metricsOverview}<ChevronDownIcon class="size-4 text-muted-foreground" aria-hidden="true" /></button>
		<Card.Root aria-hidden="true" class="hidden min-w-0 grid-cols-2 gap-px bg-border py-0 sm:grid lg:grid-cols-4">
			{#each [0, 1, 2, 3] as column (column)}
				<div class="flex min-w-0 flex-col gap-3 bg-card p-4">
					<Skeleton class="h-4 w-24" /><Skeleton class="h-8 w-28" /><Skeleton class="h-1.5 w-full" />
					<div class="grid gap-1.5">{#each [0, 1, 2] as row (row)}<div class="flex h-4 items-center justify-between gap-3"><Skeleton class="h-3 w-20" /><Skeleton class="h-3 w-8" /></div>{/each}</div>
				</div>
			{/each}
		</Card.Root>
	</div>
	<UnderlineTabs.Root value="relationships" class="min-w-0 gap-3">
		<UnderlineTabs.List class="max-w-full overflow-x-auto">{#each tabs as tab (tab.value)}<UnderlineTabs.Trigger value={tab.value} disabled>{tab.label}</UnderlineTabs.Trigger>{/each}</UnderlineTabs.List>
		<UnderlineTabs.Content value="relationships" class="grid min-w-0 gap-3 pb-24">
			<div class="flex min-w-0 flex-wrap items-center gap-2">
				<Tabs.Root value="all"><Tabs.List><Tabs.Trigger value="all" disabled>{text.allRelationships}</Tabs.Trigger><Tabs.Trigger value="mine" disabled>{text.myRelationships}</Tabs.Trigger></Tabs.List></Tabs.Root>
				<Button disabled class="ml-auto max-sm:order-1 sm:order-last"><PlusIcon data-icon="inline-start" />{text.newRelationship}</Button>
				<div class="contents">
					<Button variant="outline" disabled class="sm:hidden">{text.filters}</Button>
					<div class="hidden sm:contents">
						{#each [text.status, text.type, text.importance, text.lastContact] as label (label)}<FilterCombobox options={[]} {label} disabled class="w-auto" />{/each}
						{#if isViewCurrencyAvailable()}<Button variant="outline" disabled aria-label={text.viewCurrency}><Skeleton class="h-4 w-8" /><ChevronDownIcon class="opacity-50" /></Button>{/if}
					</div>
				</div>
			</div>
			<div aria-hidden="true" class="min-w-0 max-w-full overflow-hidden rounded-lg border bg-card shadow-sm">
				<Table.Root class="table-auto">
					<Table.Header class="bg-muted/50 text-left"><Table.Row class="hover:bg-transparent">
						<Table.Head class="w-full pl-4">{@render sortableHeading(text.organizationName)}</Table.Head>
						<Table.Head class="hidden whitespace-nowrap md:table-cell">{text.type}</Table.Head>
						<Table.Head class="whitespace-nowrap">{@render sortableHeading(text.status)}</Table.Head>
						<Table.Head class="hidden whitespace-nowrap sm:table-cell">{@render sortableHeading(text.internalOwner)}</Table.Head>
						<Table.Head class="hidden whitespace-nowrap xl:table-cell">{text.externalContact}</Table.Head>
						<Table.Head class="hidden whitespace-nowrap text-right lg:table-cell">{@render sortableHeading(text.lastContact)}</Table.Head>
						<Table.Head class="hidden whitespace-nowrap text-right xl:table-cell">{@render sortableHeading(text.nextAction)}</Table.Head>
						<Table.Head class="whitespace-nowrap pr-6 text-right">{@render sortableHeading(text.kpiOpenValue)}</Table.Head>
					</Table.Row></Table.Header>
					<Table.Body>
						{#each [0, 1, 2, 3, 4, 5] as row (row)}
							<Table.Row>
								<Table.Cell class="w-full whitespace-normal pl-4"><div class="flex h-5 items-center"><Skeleton class="h-4 w-3/4 max-w-48" /></div><div class="mt-1 hidden h-5 items-center sm:flex"><Skeleton class="h-3 w-full max-w-64" /></div></Table.Cell>
								<Table.Cell class="hidden pl-0 md:table-cell"><Skeleton class="h-5 w-8 rounded-full" /></Table.Cell>
								<Table.Cell><Skeleton class="h-5 w-12 rounded-full" /></Table.Cell>
								<Table.Cell class="hidden sm:table-cell"><Skeleton class="h-5 w-14 rounded-full" /></Table.Cell>
								<Table.Cell class="hidden xl:table-cell"><Skeleton class="h-4 w-8" /></Table.Cell>
								<Table.Cell class="hidden lg:table-cell"><Skeleton class="ml-auto h-4 w-12" /></Table.Cell>
								<Table.Cell class="hidden xl:table-cell"><Skeleton class="ml-auto h-4 w-8" /></Table.Cell>
								<Table.Cell class="pr-6"><Skeleton class="ml-auto h-4 w-16" /></Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
				<div class="flex flex-wrap items-center justify-between gap-2 border-t bg-card px-3 py-3"><Skeleton class="h-3 w-24" /><Skeleton class="h-11 w-36 sm:h-8" /></div>
			</div>
		</UnderlineTabs.Content>
	</UnderlineTabs.Root>
</div>
