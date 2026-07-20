<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { UserRecord } from '$lib/orgchart/types';
	import type { OrgchartOrganizationSection } from './orgchart-organization-model';
	import type { orgchartDirectoryText } from './text';

	let {
		section,
		selectedUserID,
		text,
		selectRecord
	}: {
		section: OrgchartOrganizationSection;
		selectedUserID: string;
		text: typeof orgchartDirectoryText.ko;
		selectRecord: (record: UserRecord) => void;
	} = $props();

	const indentation = $derived(Math.min(section.depth, 8) * 24);

	function personLabel(record: UserRecord): string {
		return record.name || record.email;
	}
</script>

<section
	class={['relative min-w-0', section.depth > 0 && 'border-l border-border pl-4']}
	style={`margin-left: ${indentation}px`}
	data-testid={`orgchart-organization-section-${section.id || 'root'}`}
>
	<div class="flex h-11 items-center justify-between rounded-lg bg-muted px-3 text-sm font-semibold">
		<span class="truncate">{section.name}</span>
		<span class="shrink-0 text-muted-foreground">{section.memberCount}{text.memberCountUnit}</span>
	</div>
	<div class="grid gap-2 pt-2" data-testid={`orgchart-organization-members-${section.id || 'root'}`}>
		{#each section.records as record (record.userID)}
			<button
				type="button"
				class={[
					'grid min-h-16 w-full min-w-0 grid-cols-[minmax(0,1fr)_minmax(96px,0.45fr)_7rem] items-center gap-3 rounded-xl border bg-card px-4 py-3 text-left text-sm transition hover:bg-muted/30',
					selectedUserID === record.userID && 'ring-1 ring-primary'
				]}
				onclick={() => selectRecord(record)}
				data-testid={`orgchart-person-node-${record.userID}`}
			>
				<span class="flex min-w-0 items-center gap-3">
					<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class="size-9 shrink-0" />
					<span class="min-w-0">
						<span class="block truncate font-semibold">{personLabel(record)}</span>
						<span class="block truncate text-muted-foreground">{record.jobTitle || text.noTitle}</span>
					</span>
				</span>
				<span class="truncate text-muted-foreground">{section.name}</span>
				{#if record.userID === section.companyResponsibleUserID}
					<span class="justify-self-end rounded-full bg-primary/10 px-2.5 py-1 text-xs font-semibold text-primary">{text.companyResponsible}</span>
				{:else if record.userID === section.responsibleUserID}
					<span class="justify-self-end rounded-full bg-primary/10 px-2.5 py-1 text-xs font-semibold text-primary">{text.organizationResponsible}</span>
				{:else}
					<span></span>
				{/if}
			</button>
		{/each}
	</div>
</section>
