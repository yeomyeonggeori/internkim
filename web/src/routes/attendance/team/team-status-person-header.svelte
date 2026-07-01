<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { TeamStatusPersonRow } from './team-status-table-model';

	type Props = {
		row: TeamStatusPersonRow;
	};

	let { row }: Props = $props();

	function locationIndicatorColor(color: string | undefined): string {
		return color ?? 'hsl(var(--muted-foreground))';
	}
</script>

<div class="sticky left-0 z-10 min-w-0 border-r bg-card px-1.5 py-1.5 sm:px-2 sm:py-2" role="rowheader">
	<div class="flex min-w-0 items-center gap-1.5 sm:gap-2">
		<PersonAvatar
			name={row.displayName}
			email={row.email}
			seed={row.email || row.mattermostUsername || row.displayName}
			class="size-5 shrink-0 sm:size-6"
		/>
		<div class="min-w-0 flex-1">
			<div class="whitespace-normal break-all text-sm font-medium leading-tight text-foreground">{row.displayName}</div>
			{#if row.currentLocationName}
				<div class="mt-0.5 hidden min-w-0 items-start gap-1 text-[11px] leading-tight text-foreground sm:flex">
					<span
						class="mt-1 size-1.5 shrink-0 rounded-full"
						style:background-color={locationIndicatorColor(row.currentLocationColor)}
					></span>
					<span class="min-w-0 whitespace-normal break-all">{row.currentLocationName}</span>
				</div>
			{/if}
		</div>
	</div>
</div>
