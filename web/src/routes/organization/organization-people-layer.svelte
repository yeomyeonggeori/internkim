<script lang="ts">
	import type { UserRecord } from '$lib/organization/types';
	import OrganizationPeopleGroup from './organization-people-group.svelte';
	import type { OrganizationOrganizationSection } from './organization-model';
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
</script>

<div class="grid pb-6" data-testid="organization-people-layer">
	{#each sections as section (section.id)}
		<OrganizationPeopleGroup {section} {selectedUserID} {text} {selectRecord} />
	{/each}
</div>
