<script lang="ts">
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { buildTaskTabs } from './task-workspace-model';

	type TaskTabLabels = {
		report: string;
		tasks: string;
		definitions: string;
		members: string;
	};

	type Props = {
		activeTab: string;
		labels: TaskTabLabels;
		onSelectTab: (value: string) => void;
	};

	let { activeTab, labels, onSelectTab }: Props = $props();

	let tabs = $derived(buildTaskTabs());
</script>

<UnderlineTabs.Root value={activeTab} onValueChange={onSelectTab} data-task-active-tab={activeTab}>
	<UnderlineTabs.List>
		{#each tabs as tab (tab)}
			<UnderlineTabs.Trigger value={tab}>{labels[tab]}</UnderlineTabs.Trigger>
		{/each}
	</UnderlineTabs.List>
</UnderlineTabs.Root>
