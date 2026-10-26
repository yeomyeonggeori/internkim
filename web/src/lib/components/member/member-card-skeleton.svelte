<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
</script>

<div class="bg-card relative flex min-w-0 flex-col items-center overflow-hidden rounded-xl border text-center" data-member-skeleton>
	<div class="absolute inset-x-2 top-2 flex items-start justify-between gap-1"><Skeleton class="h-5 w-14 rounded-full" /></div>
	<div class="flex w-full min-w-0 flex-col items-center gap-2 px-4 pt-8 pb-3">
		<Skeleton class="size-16 rounded-full" />
		<span class="grid w-full min-w-0 justify-items-center gap-0.5">
			<span class="flex h-5 items-center"><Skeleton class="h-3.5 w-20" /></span>
			<span class="flex h-4 items-center"><Skeleton class="h-3 w-12" /></span>
		</span>
	</div>
	<div class="h-10 w-full border-t"></div>
</div>
