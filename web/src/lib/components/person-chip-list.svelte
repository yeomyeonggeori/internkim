<script lang="ts">
	import PersonChip from '$lib/components/person-chip.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';

	type Props = {
		names: string[];
		memberIDs: string[];
		memberEmail?: (memberID: string) => string;
		class?: string;
	};

	let { names, memberIDs, memberEmail, class: className }: Props = $props();
</script>

<div class={cn('flex flex-wrap gap-1', className)}>
	{#each names as name, index}
		{@const memberID = memberIDs[index] ?? ''}
		<PersonChip name={displayPersonName(name)} email={memberEmail?.(memberID) ?? ''} seed={memberID || name} />
	{/each}
</div>
