<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import * as Tabs from '$lib/components/ui/tabs';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import type { CRMText } from './text';
	let { text, tabs }: { text: CRMText; tabs: { value: string; label: string }[] } = $props();
</script>

<div role="status" aria-label={text.loading} aria-busy="true" class="grid min-w-0 gap-3" data-testid="crm-loading-skeleton">
	<div class="grid min-w-0 gap-3">
		<Button variant="outline" disabled class="min-h-11 w-full justify-between sm:hidden">{text.metricsOverview}<span aria-hidden="true">+</span></Button>
		<div aria-hidden="true" class="hidden overflow-hidden rounded-xl border sm:grid sm:grid-cols-2 lg:grid-cols-4">
			{#each [0, 1, 2, 3] as column (column)}
				<div class="grid gap-3 border-r bg-card p-4 last:border-r-0">
					<Skeleton class="h-3 w-24" /><Skeleton class="h-8 w-28" /><Skeleton class="h-1.5 w-full" />
					{#each [0, 1, 2] as row (row)}<div class="flex justify-between gap-3"><Skeleton class="h-3 w-20" /><Skeleton class="h-3 w-8" /></div>{/each}
				</div>
			{/each}
		</div>
		<UnderlineTabs.Root value="relationships" class="min-w-0"><UnderlineTabs.List class="max-w-full overflow-x-auto">{#each tabs as tab (tab.value)}<UnderlineTabs.Trigger value={tab.value} disabled>{tab.label}</UnderlineTabs.Trigger>{/each}</UnderlineTabs.List></UnderlineTabs.Root>
		<div class="flex min-w-0 flex-wrap items-center gap-2"><Tabs.Root value="all"><Tabs.List><Tabs.Trigger value="all" disabled>{text.allRelationships}</Tabs.Trigger><Tabs.Trigger value="mine" disabled>{text.myRelationships}</Tabs.Trigger></Tabs.List></Tabs.Root><Button disabled class="ml-auto sm:order-last"><PlusIcon data-icon="inline-start" />{text.newRelationship}</Button><Button variant="outline" disabled class="sm:hidden">{text.filters}</Button>{#each [text.status, text.type, text.importance, text.lastContact] as label (label)}<Button variant="outline" disabled class="hidden sm:inline-flex">{label}</Button>{/each}</div>
		<div aria-hidden="true" class="overflow-hidden rounded-lg border bg-card">
			<div class="grid grid-cols-[minmax(0,1fr)_4rem_5rem] gap-4 border-b bg-muted/40 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_5rem_6rem_5rem] md:grid-cols-[minmax(0,1fr)_4rem_5rem_6rem_5rem] lg:grid-cols-[minmax(0,1fr)_4rem_5rem_6rem_6rem_5rem] xl:grid-cols-[minmax(0,1fr)_3rem_4rem_5rem_5rem_6rem_5rem_5rem]">
				<Skeleton class="h-4 w-20" /><Skeleton class="hidden h-4 w-12 md:block" /><Skeleton class="h-4 w-12" /><Skeleton class="hidden h-4 w-14 sm:block" /><Skeleton class="hidden h-4 w-14 xl:block" /><Skeleton class="hidden h-4 w-16 lg:block" /><Skeleton class="hidden h-4 w-16 xl:block" /><Skeleton class="h-4 w-16" />
			</div>
			{#each [0, 1, 2, 3, 4, 5] as row (row)}
				<div class="grid grid-cols-[minmax(0,1fr)_4rem_5rem] items-center gap-4 border-b px-4 py-3 last:border-b-0 sm:grid-cols-[minmax(0,1fr)_5rem_6rem_5rem] md:grid-cols-[minmax(0,1fr)_4rem_5rem_6rem_5rem] lg:grid-cols-[minmax(0,1fr)_4rem_5rem_6rem_6rem_5rem] xl:grid-cols-[minmax(0,1fr)_3rem_4rem_5rem_5rem_6rem_5rem_5rem]">
					<div class="grid gap-2"><Skeleton class="h-4 w-3/4 max-w-48" /><Skeleton class="hidden h-3 w-full max-w-64 sm:block" /></div><Skeleton class="hidden h-5 w-12 md:block" /><Skeleton class="h-5 w-12 rounded-full" /><Skeleton class="hidden h-4 w-14 sm:block" /><Skeleton class="hidden h-3 w-14 xl:block" /><Skeleton class="hidden h-3 w-16 lg:block" /><Skeleton class="hidden h-3 w-16 xl:block" /><Skeleton class="ml-auto h-4 w-16" />
				</div>
			{/each}
			<div class="flex items-center justify-between gap-3 px-3 py-3"><Skeleton class="h-3 w-24" /><div class="flex items-center gap-2"><Skeleton class="h-7 w-12" /><Skeleton class="size-7" /><Skeleton class="h-7 w-12" /></div></div>
		</div>
	</div>
</div>
