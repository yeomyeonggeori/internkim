<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
</script>

<div class="flex min-w-0 items-center gap-2 rounded-lg px-2 py-1.5" data-member-skeleton>
	<span class="flex min-h-11 min-w-0 flex-1 items-center gap-3">
		<Skeleton class="size-9 shrink-0 rounded-full" />
		<span class="flex h-5 items-center"><Skeleton class="h-3.5 w-28" /></span>
	</span>
	<span class="h-9 w-[108px] shrink-0"></span>
</div>
