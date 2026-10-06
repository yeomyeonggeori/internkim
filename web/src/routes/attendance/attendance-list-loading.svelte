<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from './text';

	let { kind, rowCount = 3, isUnlimited = false }: {
		kind: 'leave-history' | 'approvals' | 'changes' | 'leave-employees';
		rowCount?: number;
		isUnlimited?: boolean;
	} = $props();
	const text = createPageText(attendanceText);
</script>

{#snippet identity(size: 'small' | 'medium' | 'large' = 'medium')}
	<div class="flex min-w-0 items-center gap-3">
		<Skeleton class={`${size === 'large' ? 'size-10' : size === 'small' ? 'size-7' : 'size-8'} shrink-0 rounded-full`} />
		<div class="flex h-9 min-w-0 flex-col justify-center gap-1"><Skeleton class="h-4 w-24 max-w-full" /><Skeleton class="h-3 w-36 max-w-full" /></div>
	</div>
{/snippet}

{#snippet leaveDescription(withStatus = true)}
	<div class="min-w-0 space-y-1.5">
		<Skeleton class="h-5 w-28 max-w-full" />
		<div class="flex h-5 items-center gap-2"><Skeleton class={withStatus ? 'h-4 w-8' : 'h-5 w-8'} /><Skeleton class="h-4 w-6" /><Skeleton class="h-4 w-5" />{#if withStatus}<Skeleton class="h-5 w-14" />{/if}</div>
		<Skeleton class="h-4 w-24 max-w-full" />
	</div>
{/snippet}

{#snippet changeValue()}
	<div class="grid justify-items-start gap-1.5"><div class="grid gap-0.5"><Skeleton class="h-4 w-20" /><Skeleton class="h-5 w-12" /></div><Skeleton class="h-5 w-14" /></div>
{/snippet}

<div role="status" aria-busy="true" aria-label={text.loading} data-testid="attendance-list-loading" data-loading-kind={kind}>
	<div aria-hidden="true">
		{#if kind === 'leave-history'}
			<div class="grid gap-4">
				<div class="divide-y">
					{#each Array.from({ length: rowCount }) as _, row (row)}
						<div class="flex min-w-0 items-start justify-between gap-3 py-4" data-loading-row>
							{@render leaveDescription()}<Skeleton class="h-11 w-16 shrink-0 sm:h-7" />
						</div>
					{/each}
				</div>
				<div class="flex flex-wrap items-center justify-between gap-2" data-loading-pagination><Skeleton class="h-4 w-24" /><div class="flex items-center gap-1"><Skeleton class="h-11 w-14 sm:h-8" /><Skeleton class="h-11 w-8 sm:h-8" /><Skeleton class="h-11 w-14 sm:h-8" /></div></div>
			</div>
		{:else if kind === 'approvals'}
			<div class="divide-y">
				{#each Array.from({ length: rowCount }) as _, row (row)}
					<div class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-x-4 gap-y-3 py-4 sm:grid-cols-[11rem_minmax(0,1fr)_auto]" data-loading-row>
						<div class="col-span-2 sm:col-span-1">{@render identity()}</div>
						{@render leaveDescription(false)}
						<div class="flex items-center gap-1.5 self-center"><Skeleton class="h-11 w-11 sm:h-7 sm:w-10" /><Skeleton class="h-11 w-11 sm:h-7 sm:w-10" /></div>
					</div>
				{/each}
			</div>
		{:else if kind === 'changes'}
			<div class="divide-y sm:hidden">
				{#each Array.from({ length: rowCount }) as _, row (row)}
					<div class="grid min-w-0 gap-3 py-4" data-loading-row>
						<div class="flex items-start justify-between gap-3"><div><div class="flex items-center gap-2"><Skeleton class="size-7 rounded-full" /><Skeleton class="h-5 w-20" /></div><Skeleton class="ml-9 h-4 w-20" /></div><Skeleton class="size-11" /></div>
						<div class="grid grid-cols-2 gap-3 text-sm">
							<div><div class="text-xs text-muted-foreground">{text.handWritten.before}</div>{@render changeValue()}</div>
							<div><div class="text-xs text-muted-foreground">{text.handWritten.now}</div>{@render changeValue()}</div>
							<div class="col-span-2"><div class="text-xs text-muted-foreground">{text.handWritten.reason}</div><Skeleton class="h-5 w-32" /></div>
						</div>
					</div>
				{/each}
			</div>
			<div class="hidden sm:block">
				<Table.Root class="w-full table-fixed"><Table.Header><Table.Row><Table.Head class="w-[28%] px-4">{text.handWritten.person}</Table.Head><Table.Head class="w-[22%] px-4">{text.handWritten.before}</Table.Head><Table.Head class="w-[22%] px-4">{text.handWritten.now}</Table.Head><Table.Head class="px-4">{text.handWritten.reason}</Table.Head><Table.Head class="w-12" /></Table.Row></Table.Header><Table.Body>
					{#each Array.from({ length: rowCount }) as _, row (row)}<Table.Row data-loading-row><Table.Cell class="px-4 py-4 align-top">{@render identity('small')}</Table.Cell><Table.Cell class="px-4 py-4 align-top">{@render changeValue()}</Table.Cell><Table.Cell class="px-4 py-4 align-top">{@render changeValue()}</Table.Cell><Table.Cell class="px-4 py-4 align-top"><Skeleton class="h-5 w-24 max-w-full" /></Table.Cell><Table.Cell class="text-right"><Skeleton class="ml-auto size-7" /></Table.Cell></Table.Row>{/each}
				</Table.Body></Table.Root>
			</div>
		{:else}
			<div class="divide-y sm:hidden">
				{#each Array.from({ length: rowCount }) as _, row (row)}
					<div class="grid min-w-0 gap-3 py-3" data-loading-row><div class="flex min-h-11 items-center">{@render identity('large')}</div><div class="grid grid-cols-2 gap-2 text-sm">
						{#each (isUnlimited ? [text.management.used, text.management.pending] : [text.management.available, text.management.used, text.management.pending]) as label (label)}<div><div class="text-xs text-muted-foreground">{label}</div><Skeleton class="h-5 w-12" /></div>{/each}
					</div></div>
				{/each}
			</div>
			<div class="hidden sm:block"><Table.Root><Table.Header><Table.Row><Table.Head>{text.management.employee}</Table.Head><Table.Head class="text-right">{text.management.used}</Table.Head><Table.Head class="text-right">{text.management.pending}</Table.Head>{#if !isUnlimited}<Table.Head class="text-right">{text.management.available}</Table.Head>{/if}</Table.Row></Table.Header><Table.Body>
				{#each Array.from({ length: rowCount }) as _, row (row)}<Table.Row data-loading-row><Table.Cell>{@render identity()}</Table.Cell><Table.Cell><Skeleton class="ml-auto h-5 w-10" /></Table.Cell><Table.Cell><Skeleton class="ml-auto h-5 w-10" /></Table.Cell>{#if !isUnlimited}<Table.Cell><Skeleton class="ml-auto h-5 w-10" /></Table.Cell>{/if}</Table.Row>{/each}
			</Table.Body></Table.Root></div>
		{/if}
	</div>
</div>
