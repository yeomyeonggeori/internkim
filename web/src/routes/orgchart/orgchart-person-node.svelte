<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { UserRecord } from '../admin/admin-types';
	import type { orgchartDirectoryText } from './text';

	type OrgchartPersonNodeProps = {
		record: UserRecord;
		isSelected: boolean;
		isRoot?: boolean;
		isAttachedToHeader?: boolean;
		text: typeof orgchartDirectoryText.ko;
		selectRecord: (record: UserRecord, anchor: DOMRect) => void;
	};

	let {
		record,
		isSelected,
		isRoot = false,
		isAttachedToHeader = false,
		text,
		selectRecord
	}: OrgchartPersonNodeProps = $props();

	const personName = $derived(record.name || record.email);
	const subtitle = $derived(record.jobTitle || text.noTitle);
</script>

<button
	type="button"
	class={[
		'grid h-24 min-w-0 grid-cols-[40px_minmax(0,1fr)] items-center gap-3 bg-background p-3 text-left transition',
		isAttachedToHeader ? 'rounded-b-md rounded-t-none border hover:border-primary/50 hover:bg-muted/40' : 'rounded-md border hover:border-primary/50 hover:bg-muted/40',
		isRoot ? 'w-full max-w-80' : 'w-full',
		isSelected ? 'border-primary ring-1 ring-primary' : 'border-border'
	]}
	onclick={(event) => {
		event.stopPropagation();
		selectRecord(record, event.currentTarget.getBoundingClientRect());
	}}
	aria-pressed={isSelected}
	data-testid={`orgchart-person-node-${record.userID}`}
>
	<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class="size-10" />
	<span class="grid min-w-0 gap-1">
		<span class="truncate text-sm font-semibold">{personName}</span>
		<span class="truncate text-xs text-muted-foreground">{subtitle}</span>
	</span>
</button>
