<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { UserRecord } from '$lib/organization/types';

	let { records, memberCountUnit }: { records: UserRecord[]; memberCountUnit: string } = $props();
	const visibleRecords = $derived(records.slice(0, 3));
	const remainingCount = $derived(Math.max(records.length - visibleRecords.length, 0));
</script>

<div class="flex shrink-0 items-center -space-x-2" aria-label={`${records.length}${memberCountUnit}`} data-testid="organization-avatar-stack">
	{#each visibleRecords as record (record.userID)}
		<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class="size-7 border-2 border-background" />
	{/each}
	{#if remainingCount > 0}
		<span class="border-background bg-muted text-muted-foreground relative z-10 grid size-7 place-items-center rounded-full border-2 text-[10px] font-semibold">+{remainingCount}</span>
	{/if}
</div>
