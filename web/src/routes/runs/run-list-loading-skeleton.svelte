<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	let { label, isAdmin = false }: { label: string; isAdmin?: boolean } = $props();
</script>

<Card.Root role="status" aria-label={label} aria-busy="true" class="min-w-0" data-testid="run-list-loading-skeleton">
	<Card.Content class="px-0">
		<div aria-hidden="true" class="divide-y md:hidden">
			{#each [0, 1, 2, 3, 4] as row (row)}
				<div class="flex flex-col gap-2 px-4 py-3">
					<div class="flex min-w-0 items-start justify-between gap-3">
						<div class="min-w-0 flex-1">
							<div class="flex h-5 items-center"><Skeleton class="h-4 w-4/5" /></div>
							{#if isAdmin}
								<div class="mt-1 flex h-5 items-center gap-2"><Skeleton class="size-5 shrink-0 rounded-full" /><Skeleton class="h-3 w-24" /></div>
							{/if}
						</div>
					<Skeleton class="h-5 w-16 shrink-0" />
					</div>
					<div class="flex h-11 items-center justify-between gap-3 sm:h-6"><Skeleton class="h-3 w-28" /><Skeleton class="h-3 w-12" /></div>
				</div>
			{/each}
		</div>
		<div aria-hidden="true" class="hidden md:block">
			<Table.Root>
				<Table.Header>
					<Table.Row class="hover:bg-transparent">
						<Table.Head><Skeleton class="h-4 w-24" /></Table.Head>
						{#if isAdmin}<Table.Head class="w-44"><Skeleton class="h-4 w-20" /></Table.Head>{/if}
						<Table.Head class="w-32"><Skeleton class="h-4 w-16" /></Table.Head>
						<Table.Head class="w-28"><Skeleton class="ml-auto h-4 w-14" /></Table.Head>
						<Table.Head class="w-44"><Skeleton class="ml-auto h-4 w-20" /></Table.Head>
						<Table.Head class="w-12"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each [0, 1, 2, 3, 4, 5] as row (row)}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell><div class="flex h-5 items-center"><Skeleton class="h-4 w-4/5" /></div></Table.Cell>
							{#if isAdmin}<Table.Cell><div class="flex h-6 items-center gap-2"><Skeleton class="size-6 shrink-0 rounded-full" /><Skeleton class="h-4 w-24" /></div></Table.Cell>{/if}
							<Table.Cell><Skeleton class="h-5 w-20" /></Table.Cell>
							<Table.Cell><Skeleton class="ml-auto h-3 w-12" /></Table.Cell>
							<Table.Cell><Skeleton class="ml-auto h-3 w-28" /></Table.Cell>
							<Table.Cell><Skeleton class="ml-auto size-6" /></Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	</Card.Content>
</Card.Root>
