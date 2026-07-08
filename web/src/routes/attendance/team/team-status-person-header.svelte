<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import LocationLabel from '../shared/location-label.svelte';
	import type { TeamStatusPersonRow } from './team-status-table-model';

	type Props = {
		row: TeamStatusPersonRow;
		columnIndex: number;
		isLastColumn: boolean;
	};

	let { row, columnIndex, isLastColumn }: Props = $props();

	const dividerClass = $derived(
		`${columnIndex === 0 ? '' : 'border-l'} ${isLastColumn ? 'border-r' : ''}`
	);
</script>

<div class={`flex min-w-0 flex-col justify-start gap-1 px-2 py-2 ${dividerClass}`} role="columnheader">
	<div class="flex min-w-0 items-center gap-2">
		<PersonAvatar
			name={row.displayName}
			email={row.email}
			seed={row.email || row.mattermostUsername || row.displayName}
			class="size-7 shrink-0"
		/>
		<div class="min-w-0 flex-1 whitespace-normal break-all text-sm font-medium leading-tight text-foreground">
			{row.displayName}
		</div>
	</div>
	{#if row.currentLocationName}
		<div class="flex min-w-0 justify-end" data-testid="team-status-current-location">
			<LocationLabel name={row.currentLocationName} class="max-w-full" />
		</div>
	{/if}
</div>
