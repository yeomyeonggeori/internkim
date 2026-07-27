<script lang="ts">
	import type { UserRecord } from '$lib/organization/types';
	import OrganizationPeopleGroup from './organization-people-group.svelte';
	import type { OrganizationOrganizationSection } from './organization-model';
	import { organizationSectionsWithRecords, organizationSectionTree } from './organization-section-tree';
	import type { organizationDirectoryText } from './text';

	let {
		sections,
		selectedUserID,
		hidesEmptySections = false,
		text,
		selectRecord
	}: {
		sections: OrganizationOrganizationSection[];
		selectedUserID: string;
		hidesEmptySections?: boolean;
		text: typeof organizationDirectoryText.ko;
		selectRecord: (record: UserRecord) => void;
	} = $props();

	const tree = $derived(organizationSectionTree(sections));
	const nodes = $derived(hidesEmptySections ? organizationSectionsWithRecords(tree) : tree);
</script>

<div class="grid pt-4 pb-6" data-testid="organization-people-layer">
	{#each nodes as node (node.section.id)}
		<OrganizationPeopleGroup {node} {selectedUserID} {text} {selectRecord} />
	{/each}
</div>
