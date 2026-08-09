<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';

	let { organizationCount = 5, personCount = 8 }: { organizationCount?: number; personCount?: number } = $props();
</script>

<div class="grid h-full min-h-0 lg:grid-cols-[270px_minmax(0,1fr)]" data-testid="organization-skeleton" aria-hidden="true">
	<div class="hidden min-h-0 space-y-2 border-r p-4 lg:block">
		<Skeleton class="mb-4 h-5 w-24" />
		{#each Array.from({ length: organizationCount }) as _, organizationIndex (organizationIndex)}
			<Skeleton class="h-8" style="width: {70 + ((organizationIndex * 13) % 30)}%" />
		{/each}
	</div>
	<div class="min-h-0 space-y-4 p-6">
		<div class="flex items-center gap-2">
			<Skeleton class="h-9 w-48" />
			<Skeleton class="ml-auto h-9 w-24" />
		</div>
		<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
			{#each Array.from({ length: personCount }) as _, personIndex (personIndex)}
				<div class="flex items-center gap-3 rounded-xl border p-4">
					<Skeleton class="size-10 shrink-0 rounded-full" />
					<div class="min-w-0 flex-1 space-y-2">
						<Skeleton class="h-4 w-24" />
						<Skeleton class="h-3 w-32" />
					</div>
				</div>
			{/each}
		</div>
	</div>
</div>
