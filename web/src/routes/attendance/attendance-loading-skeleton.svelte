<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from './text';
	let { kind = 'dashboard', rowCount = 6 }: {
		kind?: 'dashboard' | 'metrics' | 'teams' | 'members' | 'month' | 'records';
		rowCount?: number;
	} = $props();
	const text = createPageText(attendanceText);
</script>

{#snippet metrics()}
	<div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
		{#each Array.from({ length: 4 }) as _, index (index)}
			<Card.Root class="gap-2 py-4">
				<Card.Header class="px-4"><Card.Description><Skeleton class="h-5 w-20" /></Card.Description></Card.Header>
				<Card.Content class="flex items-end justify-between px-4"><Skeleton class="h-9 w-20" /><Skeleton class="h-5 w-9" /></Card.Content>
			</Card.Root>
		{/each}
	</div>
{/snippet}

{#snippet teams()}
	<div class="grid gap-4 min-[761px]:grid-cols-2 min-[1101px]:grid-cols-3">
		{#each Array.from({ length: rowCount }) as _, index (index)}
			<Card.Root class="gap-0 overflow-hidden rounded-xl py-0">
				<Card.Header class="gap-3 px-4 pb-3 pt-4">
					<div class="flex justify-between gap-3"><div class="space-y-2"><Skeleton class="h-5 w-24" /><Skeleton class="h-4 w-12" /></div><div class="space-y-2"><Skeleton class="ml-auto h-6 w-14" /><Skeleton class="h-4 w-20" /></div></div>
					<Skeleton class="h-3 w-full rounded-full" />
				</Card.Header>
				<Card.Content class="flex flex-col gap-4 px-4 pb-4">
					<div class="grid grid-cols-2 gap-x-6 gap-y-2">{#each Array.from({ length: 4 }) as _, metric (metric)}<Skeleton class="h-4 w-full" />{/each}</div>
					<div class="border-t border-border/70 pt-3"><Skeleton class="h-4 w-3/4" /></div>
					<div class="flex flex-col gap-3 border-t border-border/70 pt-3">{#each [0, 1] as row (row)}<div class="flex h-8 items-center justify-between"><Skeleton class="h-4 w-20" /><div class="flex -space-x-2">{#each [0, 1, 2] as avatar (avatar)}<Skeleton class="size-8 rounded-full ring-2 ring-card" />{/each}</div></div>{/each}</div>
				</Card.Content>
				<Card.Footer class="h-11 border-t border-border/70 px-4"><Skeleton class="h-4 w-24" /></Card.Footer>
			</Card.Root>
		{/each}
	</div>
{/snippet}

<div role="status" aria-busy="true" aria-label={text.loading} data-testid="attendance-skeleton" data-attendance-skeleton={kind}>
	<div aria-hidden="true" class="flex flex-col gap-6">
		{#if kind === 'dashboard'}
			<Card.Root class="gap-0 py-0"><Card.Content class="py-4 sm:py-3">
				<div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3">
					<div class="col-span-2 flex items-center gap-3 sm:col-span-1"><Skeleton class="size-9 rounded-full" /><div class="space-y-1"><Skeleton class="h-5 w-24" /><Skeleton class="h-5 w-32" /></div></div>
					<Skeleton class="order-3 col-span-2 mt-4 h-11 w-full sm:order-none sm:col-span-1 sm:mt-0 sm:w-56" />
					<Skeleton class="order-2 col-span-2 mt-4 h-1.5 w-full rounded-full sm:order-3 sm:mt-3" />
				</div>
			</Card.Content></Card.Root>
			{@render metrics()}
			<div class="flex items-end justify-between gap-3"><Skeleton class="h-7 w-32" /><Skeleton class="h-4 w-20" /></div>
			{@render teams()}
		{:else if kind === 'metrics'}
			{@render metrics()}
		{:else if kind === 'teams'}
			{@render teams()}
		{:else if kind === 'month'}
			<Card.Root class="min-w-0 gap-4"><Card.Header class="flex min-w-0 flex-col gap-[10px] overflow-hidden pb-1 sm:flex-row sm:items-start sm:justify-between"><div class="flex w-full min-w-0 items-center justify-between gap-2 sm:w-auto"><Card.Title class="min-w-0 flex-1 truncate text-base sm:flex-none sm:whitespace-nowrap">{text.teamMonthlyStatus}</Card.Title><Skeleton class="h-11 w-36 shrink-0 sm:hidden" /></div><div class="flex w-full min-w-0 flex-col gap-[10px] sm:w-auto"><Skeleton class="hidden h-8 w-40 sm:block" /></div></Card.Header><Card.Content class="overflow-hidden">
				<div class="overflow-hidden rounded-md border"><div class="grid grid-cols-[4.5rem_minmax(0,1fr)] border-b bg-muted"><div class="border-r"></div><div class="flex flex-col gap-1 px-2 py-2"><div class="flex items-center gap-2"><Skeleton class="size-7 rounded-full" /><Skeleton class="h-4 w-24" /></div><Skeleton class="ml-auto h-5 w-16" /></div></div>
				<div class="divide-y">{#each Array.from({ length: rowCount }) as _, index (index)}<div class="grid h-12 grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2"><div class="px-2"><Skeleton class="h-4 w-12" /></div><div class="space-y-2 px-2"><Skeleton class="h-3 w-24" /><Skeleton class="h-1.5 w-full rounded-full" /></div></div>{/each}</div></div>
			</Card.Content></Card.Root>
		{:else}
			<div class="divide-y rounded-xl border">
				{#each Array.from({ length: rowCount }) as _, index (index)}
					<div class="space-y-2 px-6 py-3"><div class="flex items-center gap-3"><Skeleton class="size-9 shrink-0 rounded-full" /><div class="min-w-0 flex-1 space-y-2"><Skeleton class="h-5 w-24" /><Skeleton class="h-3 w-36 max-w-full" /></div><Skeleton class="h-5 w-14" /></div><div class="pl-12"><Skeleton class={kind === 'members' ? 'h-1.5 w-full rounded-full' : 'h-4 w-3/4'} /></div></div>
				{/each}
			</div>
		{/if}
	</div>
</div>
