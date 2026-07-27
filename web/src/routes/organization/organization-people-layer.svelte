<script lang="ts">
	import type { UserRecord } from '$lib/organization/types';
	import OrganizationPeopleGroup from './organization-people-group.svelte';
	import type { OrganizationOrganizationSection } from './organization-model';
	import { organizationSectionTree } from './organization-section-tree';
	import type { organizationDirectoryText } from './text';

	let {
		sections,
		selectedUserID,
		text,
		selectRecord
	}: {
		sections: OrganizationOrganizationSection[];
		selectedUserID: string;
		text: typeof organizationDirectoryText.ko;
		selectRecord: (record: UserRecord) => void;
	} = $props();

	const nodes = $derived(organizationSectionTree(sections));
</script>

<div class="grid pt-4 pb-6" data-testid="organization-people-layer">
	{#each nodes as node (node.section.id)}
		<OrganizationPeopleGroup {node} {selectedUserID} {text} {selectRecord} />
	{/each}
</div>
