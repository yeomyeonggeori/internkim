<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import type { PageText } from '$lib/i18n/page-text.svelte';
	import ListPaginationLoading from '$lib/components/list-pagination-loading.svelte';
	import type { taskText } from './text';
	let { label, variant = 'list', text }: { label: string; variant?: 'list' | 'members' | 'report' | 'definitions'; text: PageText<typeof taskText> } = $props();
</script>

{#snippet reportCard(title: string, description: string, heightClass: string, size: 'sm' | 'default')}
	<Card.Root {size} class={`${heightClass} flex min-w-0 w-full flex-col`}>
		<Card.Header class={size === 'sm' ? 'pb-1' : 'pb-3'}>
			<div class="min-w-0 space-y-1">
				<Card.Title class="text-base">{title}</Card.Title>
				<Card.Description class="text-sm leading-snug">{description}</Card.Description>
			</div>
		</Card.Header>
		<Card.Content class="min-h-0 flex-1"><Skeleton class="h-full min-h-32 w-full" /></Card.Content>
	</Card.Root>
{/snippet}

<div role="status" aria-busy="true" data-task-content-skeleton={variant}>
	<span class="sr-only">{label}</span>
	<div aria-hidden="true">
		{#if variant === 'report'}
			<div class="grid min-w-0 gap-4 lg:grid-cols-2">
				<div class="grid gap-4">
					{@render reportCard(text.report.weeklyStatus, text.report.weeklyStatusDescription, 'min-h-[23rem]', 'sm')}
					{@render reportCard(text.report.businessDistance, text.report.businessDistanceDescription, 'min-h-[18rem]', 'default')}
				</div>
				<Card.Root class="flex h-full min-h-[28rem] min-w-0 w-full flex-col">
					<Card.Header class="pb-3">
						<div class="min-w-0 space-y-1">
							<Card.Title class="text-base">{text.report.memberDistance}</Card.Title>
							<Card.Description class="text-sm leading-snug">{text.report.memberDistanceDescription}</Card.Description>
						</div>
					</Card.Header>
					<Card.Content class="min-h-0 flex-1 space-y-6">
						{#each [0, 1, 2, 3, 4, 5] as row (row)}<div class="flex items-center gap-3"><Skeleton class="size-8 shrink-0 rounded-full" /><Skeleton class="h-4 flex-1" /><Skeleton class="h-4 w-12" /></div>{/each}
					</Card.Content>
				</Card.Root>
				{@render reportCard(text.report.weeklyDistanceTrend, text.report.weeklyDistanceTrendDescription, 'min-h-[32rem]', 'default')}
				{@render reportCard(text.report.monthlyDistanceTrend, text.report.monthlyDistanceTrendDescription, 'min-h-[32rem]', 'default')}
			</div>
		{:else if variant === 'definitions'}
			<div class="grid gap-4">
				<Card.Root>
					<Card.Header>
						<Card.Title>{text.definitions.size}</Card.Title>
						<Card.Description>{text.definitions.sizeDescription}</Card.Description>
					</Card.Header>
					<Card.Content class="space-y-5">{#each [0, 1, 2, 3] as row}<Skeleton class="h-9 w-full" />{/each}</Card.Content>
				</Card.Root>
				<div class="grid gap-4 lg:grid-cols-2">
					{#each [text.definitions.business, text.definitions.type] as title (title)}
						<Card.Root size="sm">
							<Card.Header><Card.Title>{title}</Card.Title></Card.Header>
							<Card.Content class="space-y-5">{#each [0, 1, 2] as row}<Skeleton class="h-9 w-full" />{/each}</Card.Content>
						</Card.Root>
					{/each}
				</div>
			</div>
		{:else if variant === 'members'}
			<Card.Root>
				<Card.Header><Card.Title>{text.members.title}</Card.Title></Card.Header>
				<Card.Content>
					<div class="divide-y rounded-lg border sm:hidden">
						{#each [0, 1, 2, 3] as row (row)}
							<div class="grid min-w-0 gap-3 p-3" data-task-member-skeleton>
								<div class="flex min-w-0 items-start gap-3">
									<Skeleton class="size-10 shrink-0 rounded-full" />
									<div class="min-w-0 flex-1"><div class="flex h-5 items-center"><Skeleton class="h-4 w-24" /></div><div class="flex h-5 items-center"><Skeleton class="h-4 w-44 max-w-full" /></div><div class="mt-2 flex flex-wrap items-center gap-2"><Skeleton class="h-5 w-12 rounded-full" /><Skeleton class="h-3 w-28" /></div></div>
								</div>
								<div class="grid grid-cols-3 gap-2 text-sm">{#each [text.members.active, text.members.completed, text.members.distance] as heading, column (column)}<div><div class="h-4 text-xs text-muted-foreground">{heading}</div><div class="flex h-5 items-center"><Skeleton class="h-4 w-6" /></div></div>{/each}</div>
							</div>
						{/each}
					</div>
					<div class="hidden overflow-x-auto rounded-lg border sm:block">
						<Table.Root class="min-w-[760px]">
							<Table.Header class="bg-muted/40"><Table.Row class="hover:bg-transparent">{#each [text.members.name, text.members.email, text.members.hireDate, text.members.role, text.members.active, text.members.completed, text.members.distance] as heading, column (column)}<Table.Head class={`h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground ${column > 3 ? 'text-right' : ''}`}>{heading}</Table.Head>{/each}</Table.Row></Table.Header>
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
				<div class="overflow-hidden rounded-lg border bg-card">
					<Table.Root>
						<Table.Header class="bg-muted/40"><Table.Row class="hover:bg-transparent">{#each [text.table.business, text.table.type, text.table.content, text.table.participants, text.table.size, text.table.status, text.table.startDate, text.table.endDate] as heading, column (column)}<Table.Head class="h-10"><div class="flex items-center gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">{heading}{#if column !== 2 && column !== 3}<ArrowUpDownIcon class="size-3 opacity-40" />{/if}</div></Table.Head>{/each}</Table.Row></Table.Header>
						<Table.Body>{#each [0, 1, 2, 3, 4, 5] as row (row)}<Table.Row>
							<Table.Cell><Skeleton class="h-5 w-14 rounded-md" /></Table.Cell><Table.Cell><Skeleton class="h-5 w-14 rounded-md" /></Table.Cell><Table.Cell><Skeleton class="h-4 w-64 max-w-[26rem]" /></Table.Cell><Table.Cell><Skeleton class="h-5 w-24 rounded-full" /></Table.Cell><Table.Cell><Skeleton class="h-5 w-8 rounded-full" /></Table.Cell><Table.Cell><Skeleton class="h-7 w-28" /></Table.Cell><Table.Cell><Skeleton class="h-4 w-20" /></Table.Cell><Table.Cell><Skeleton class="h-4 w-20" /></Table.Cell>
						</Table.Row>{/each}</Table.Body>
					</Table.Root>
				</div>
				<ListPaginationLoading previousLabel={text.table.pagination.previous} nextLabel={text.table.pagination.next} />
			</div>
		{/if}
	</div>
</div>
