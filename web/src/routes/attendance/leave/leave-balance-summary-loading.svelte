<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';

	const text = createPageText(attendanceText);
	const columnLabels = $derived([
		text.leave.summaryUsed,
		text.leave.summaryPending,
		text.leave.summaryAvailable
	]);
</script>

<div role="status" aria-label={text.loading}>
	<div aria-hidden="true" class="space-y-2">
		<div class="grid grid-cols-[1fr_1fr_auto] gap-2">
			{#each columnLabels as label (label)}
				<div>
					<p class="text-[11px] text-muted-foreground">{label}</p>
					<Skeleton class="mt-0.5 h-5 w-12" />
				</div>
			{/each}
		</div>
		<Skeleton class="h-2 w-full rounded-full" />
	</div>
</div>
