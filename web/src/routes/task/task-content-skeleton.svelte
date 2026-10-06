<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	let { label, variant = 'list' }: { label: string; variant?: 'list' | 'members' | 'report' | 'definitions' | 'relationships' } = $props();
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
		{:else}
			<div class={variant === 'members' ? 'space-y-5 rounded-xl border bg-card p-6' : 'space-y-3'}>
				{#if variant === 'members'}<Skeleton class="h-5 w-32" />{/if}
				<div class="space-y-2 sm:hidden">
					{#if variant === 'list'}<Skeleton class="h-9 w-16" />{/if}
					{#each [0, 1, 2, 3] as row}
						<div class="space-y-4 rounded-lg border bg-card p-3"><div class="flex min-h-11 items-center gap-3">{#if variant === 'members'}<Skeleton class="size-9 shrink-0 rounded-full" />{/if}<Skeleton class="h-4 flex-1" /><Skeleton class="h-7 w-20" /></div><Skeleton class="h-4 w-20" /></div>
					{/each}
				</div>
				<div class="hidden overflow-hidden rounded-lg border bg-card sm:block">
					<div class="flex h-10 items-center gap-6 border-b bg-muted/40 px-4">{#each [0, 1, 2, 3, 4] as column}<Skeleton class="h-4 flex-1" />{/each}</div>
					{#each [0, 1, 2, 3, 4, 5] as row}
						<div class="flex h-14 items-center gap-6 border-b px-4 last:border-b-0">{#if variant === 'members'}<Skeleton class="size-8 shrink-0 rounded-full" />{/if}<Skeleton class="h-4 flex-[3]" />{#each [0, 1, 2] as column}<Skeleton class="h-4 flex-1" />{/each}</div>
					{/each}
				</div>
				<div class="flex justify-end gap-2"><Skeleton class="h-8 w-20" /><Skeleton class="h-8 w-20" /></div>
			</div>
		{/if}
	</div>
</div>
