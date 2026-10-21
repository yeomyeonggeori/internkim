<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
</script>

<div class="grid gap-10" aria-busy="true">
	<header class="grid gap-5">
		<div class="grid max-w-[68ch] gap-3">
			<Skeleton class="h-6 w-2/3" />
			<Skeleton class="h-4 w-full" />
			<Skeleton class="h-4 w-5/6" />
		</div>
		<div class="flex flex-wrap gap-5"><Skeleton class="h-4 w-32" /><Skeleton class="h-4 w-40" /></div>
		<div class="bg-border grid grid-cols-2 gap-px overflow-hidden rounded-lg border md:grid-cols-4">
			{#each [0, 1, 2, 3] as cell (cell)}
				<div class="bg-background grid gap-2 px-5 py-4"><Skeleton class="h-3 w-16" /><Skeleton class="h-7 w-24" /></div>
			{/each}
		</div>
	</header>
	<section class="grid gap-3">
		<Skeleton class="h-6 w-24" />
		<div class="bg-border grid gap-px overflow-hidden rounded-lg border sm:grid-cols-3">
			{#each [0, 1, 2] as card (card)}
				<div class="bg-background grid gap-3 p-5"><Skeleton class="h-3 w-20" /><Skeleton class="h-7 w-28" /><Skeleton class="h-16 w-full" /></div>
			{/each}
		</div>
	</section>
</div>
