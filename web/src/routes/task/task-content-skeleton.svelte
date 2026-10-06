<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import type { PageText } from '$lib/i18n/page-text.svelte';
	import type { taskText } from './text';
	let { label, variant = 'list', text }: { label: string; variant?: 'list' | 'members' | 'report' | 'definitions' | 'relationships'; text?: PageText<typeof taskText> } = $props();
</script>

<div role="status" aria-busy="true" data-task-content-skeleton={variant}>
	<span class="sr-only">{label}</span>
	<div aria-hidden="true">
		{#if variant === 'report'}
			<div class="grid min-w-0 gap-4 lg:grid-cols-2">
				<div class="grid gap-4">
					{#each ['min-h-[23rem]', 'min-h-[18rem]'] as height}
						<div class={`flex flex-col gap-4 rounded-xl border bg-card p-6 ${height}`}><Skeleton class="h-5 w-40" /><Skeleton class="h-4 w-2/3" /><Skeleton class="mt-4 min-h-32 w-full flex-1" /></div>
					{/each}
				</div>
				<div class="space-y-6 rounded-xl border bg-card p-6"><Skeleton class="h-5 w-40" />{#each [0, 1, 2, 3, 4, 5] as row}<div class="flex items-center gap-3"><Skeleton class="size-8 shrink-0 rounded-full" /><Skeleton class="h-4 flex-1" /><Skeleton class="h-4 w-12" /></div>{/each}</div>
				{#each [0, 1] as chart}<div class="flex min-h-[32rem] flex-col gap-4 rounded-xl border bg-card p-6"><Skeleton class="h-5 w-40" /><Skeleton class="h-4 w-2/3" /><Skeleton class="mt-4 w-full flex-1" /></div>{/each}
			</div>
		{:else if variant === 'definitions'}
			<div class="grid gap-4">
				<div class="space-y-5 rounded-xl border bg-card p-6"><Skeleton class="h-5 w-32" />{#each [0, 1, 2, 3] as row}<Skeleton class="h-9 w-full" />{/each}</div>
				<div class="grid gap-4 lg:grid-cols-2">{#each [0, 1] as card}<div class="space-y-5 rounded-xl border bg-card p-6"><Skeleton class="h-5 w-32" />{#each [0, 1, 2] as row}<Skeleton class="h-9 w-full" />{/each}</div>{/each}</div>
			</div>
		{:else if variant === 'relationships'}
			<div class="space-y-3"><Skeleton class="h-5 w-28" />{#each [0, 1, 2] as row}<div class="flex min-h-12 items-center gap-3 rounded-lg border p-3"><Skeleton class="size-5 shrink-0" /><Skeleton class="h-4 flex-1" /><Skeleton class="h-5 w-16" /></div>{/each}</div>
		{:else if variant === 'members'}
			<Card.Root>
				<Card.Header><Card.Title>{#if text}{text.members.title}{:else}<Skeleton class="h-[22px] w-24" />{/if}</Card.Title></Card.Header>
				<Card.Content>
					<div class="divide-y rounded-lg border sm:hidden">
						{#each [0, 1, 2, 3] as row (row)}
							<div class="grid min-w-0 gap-3 p-3" data-task-member-skeleton>
								<div class="flex min-w-0 items-start gap-3">
									<Skeleton class="size-10 shrink-0 rounded-full" />
									<div class="min-w-0 flex-1"><div class="flex h-5 items-center"><Skeleton class="h-4 w-24" /></div><div class="flex h-5 items-center"><Skeleton class="h-4 w-44 max-w-full" /></div><div class="mt-2 flex flex-wrap items-center gap-2"><Skeleton class="h-5 w-12 rounded-full" /><Skeleton class="h-3 w-28" /></div></div>
								</div>
								<div class="grid grid-cols-3 gap-2 text-sm">{#each [text?.members.active, text?.members.completed, text?.members.distance] as heading, column (column)}<div><div class="h-4 text-xs text-muted-foreground">{#if heading}{heading}{:else}<Skeleton class="h-3 w-10" />{/if}</div><div class="flex h-5 items-center"><Skeleton class="h-4 w-6" /></div></div>{/each}</div>
							</div>
						{/each}
					</div>
					<div class="hidden overflow-x-auto rounded-lg border sm:block">
						<Table.Root class="min-w-[760px]">
							<Table.Header class="bg-muted/40"><Table.Row class="hover:bg-transparent">{#each [text?.members.name, text?.members.email, text?.members.hireDate, text?.members.role, text?.members.active, text?.members.completed, text?.members.distance] as heading, column (column)}<Table.Head class={`h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground ${column > 3 ? 'text-right' : ''}`}>{#if heading}{heading}{:else}<Skeleton class="h-4 w-12" />{/if}</Table.Head>{/each}</Table.Row></Table.Header>
							<Table.Body>{#each [0, 1, 2, 3, 4, 5] as row (row)}<Table.Row>
								<Table.Cell><div class="flex items-center gap-2"><Skeleton class="size-7 shrink-0 rounded-full" /><Skeleton class="h-4 w-20" /></div></Table.Cell>
								<Table.Cell><Skeleton class="h-4 w-44" /></Table.Cell><Table.Cell><Skeleton class="h-4 w-20" /></Table.Cell><Table.Cell><Skeleton class="h-5 w-12 rounded-full" /></Table.Cell>
								{#each [0, 1, 2] as column (column)}<Table.Cell><Skeleton class="ml-auto h-4 w-6" /></Table.Cell>{/each}
							</Table.Row>{/each}</Table.Body>
						</Table.Root>
					</div>
				</Card.Content>
			</Card.Root>
		{:else}
			<div class="space-y-3">
				<div class="space-y-3 sm:hidden">
					<Button variant="outline" disabled>{#if text}{text.table.sort}{:else}<Skeleton class="h-4 w-7" />{/if}</Button>
					<div class="grid gap-2">{#each [0, 1, 2, 3] as row (row)}<div class="min-w-0 rounded-lg border bg-card p-3" data-task-list-row-skeleton><div class="flex min-w-0 items-start gap-2"><div class="flex min-h-11 min-w-0 flex-1 items-center"><Skeleton class="h-4 w-4/5" /></div><Skeleton class="h-7 w-28 shrink-0" /></div><Button variant="ghost" size="sm" disabled class="px-0 text-muted-foreground">{#if text}{text.table.details}{:else}<Skeleton class="h-4 w-16" />{/if}</Button></div>{/each}</div>
				</div>
				<div class="hidden overflow-hidden rounded-lg border bg-card sm:block">
					<Table.Root class="min-w-[1080px]">
						<Table.Header class="bg-muted/40"><Table.Row class="hover:bg-transparent">{#each [text?.table.business, text?.table.type, text?.table.content, text?.table.participants, text?.table.size, text?.table.status, text?.table.startDate, text?.table.endDate] as heading, column (column)}<Table.Head class="h-10"><div class="flex items-center gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">{#if heading}{heading}{:else}<Skeleton class="h-4 w-12" />{/if}{#if column !== 2 && column !== 3}<ArrowUpDownIcon class="size-3 opacity-40" />{/if}</div></Table.Head>{/each}</Table.Row></Table.Header>
						<Table.Body>{#each [0, 1, 2, 3, 4, 5] as row (row)}<Table.Row>
							<Table.Cell><Skeleton class="h-5 w-14 rounded-md" /></Table.Cell><Table.Cell><Skeleton class="h-5 w-14 rounded-md" /></Table.Cell><Table.Cell><Skeleton class="h-4 w-64 max-w-[26rem]" /></Table.Cell><Table.Cell><Skeleton class="h-5 w-24 rounded-full" /></Table.Cell><Table.Cell><Skeleton class="h-5 w-8 rounded-full" /></Table.Cell><Table.Cell><Skeleton class="h-7 w-28" /></Table.Cell><Table.Cell><Skeleton class="h-4 w-20" /></Table.Cell><Table.Cell><Skeleton class="h-4 w-20" /></Table.Cell>
						</Table.Row>{/each}</Table.Body>
					</Table.Root>
				</div>
				<div class="flex flex-wrap items-center justify-between gap-2"><Skeleton class="h-3 w-24" /><Skeleton class="h-11 w-36 sm:h-8" /></div>
			</div>
		{/if}
	</div>
</div>
