<script lang="ts">
	import type { UserRecord } from '$lib/orgchart/types';
	import OrgchartPeopleGroup from './orgchart-people-group.svelte';
	import type { OrgchartOrganizationSection } from './orgchart-organization-model';
	import type { orgchartDirectoryText } from './text';

	let {
		sections,
		selectedUserID,
		text,
		selectRecord
	}: {
		sections: OrgchartOrganizationSection[];
		selectedUserID: string;
		text: typeof orgchartDirectoryText.ko;
		selectRecord: (record: UserRecord) => void;
	} = $props();
</script>

<div class="grid gap-3 pb-6" data-testid="orgchart-people-layer">
	{#each sections as section (section.id)}
		<OrgchartPeopleGroup {section} {selectedUserID} {text} {selectRecord} />
	{/each}
</div>
