<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';

	let { organizationCount = 5, personCount = 8, label }: { organizationCount?: number; personCount?: number; label: string } = $props();
</script>

<div class="grid h-full min-h-0 lg:grid-cols-[270px_minmax(0,1fr)]" data-testid="organization-skeleton" role="status" aria-label={label} aria-busy="true">
	<div aria-hidden="true" class="hidden min-h-0 space-y-2 border-r p-4 lg:block">
		<Skeleton class="mb-4 h-5 w-24" />
		{#each Array.from({ length: organizationCount }) as _, organizationIndex (organizationIndex)}
			<Skeleton class="h-8" style="width: {70 + ((organizationIndex * 13) % 30)}%" />
		{/each}
	</div>
	<div aria-hidden="true" class="min-h-0 overflow-hidden">
		<div class="flex flex-wrap items-center gap-2 px-3 py-3 sm:flex-nowrap sm:px-6">
			<Skeleton class="h-9 w-9 shrink-0" /><Skeleton class="h-9 min-w-0 flex-1 sm:ml-auto sm:w-52 sm:flex-none" />
			<Skeleton class="order-first h-9 w-full sm:order-none sm:w-72" />
		</div>
		<div class="px-4 pt-4 sm:px-6"><div class="flex h-9 items-center gap-2 pl-2"><Skeleton class="size-6" /><Skeleton class="h-4 w-28" /><Skeleton class="h-4 w-8" /></div>
		<div class="grid grid-cols-1 gap-2 pl-9 pr-2 sm:grid-cols-[repeat(auto-fill,minmax(11rem,1fr))]">
			{#each Array.from({ length: personCount }) as _, personIndex (personIndex)}
				<div class="grid gap-2 rounded-xl border p-3 sm:justify-items-center sm:px-4 sm:pt-7 sm:pb-2">
					<div class="flex min-w-0 items-center gap-3 sm:grid sm:justify-items-center sm:gap-2"><Skeleton class="size-10 shrink-0 rounded-full sm:size-20" /><div class="grid min-w-0 gap-1.5 sm:justify-items-center"><Skeleton class="h-4 w-24 max-w-full" /><Skeleton class="h-3 w-28 max-w-full" /></div></div>
					<div class="flex gap-2 sm:justify-center"><Skeleton class="size-8 rounded-md" /><Skeleton class="size-8 rounded-md" /></div>
				</div>
			{/each}
		</div></div>
	</div>
</div>
