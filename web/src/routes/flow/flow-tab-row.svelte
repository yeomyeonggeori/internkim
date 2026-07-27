<script lang="ts">
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { buildFlowTaskTabs } from './flow-task-workspace-model';

	type FlowTabLabels = {
		report: string;
		tasks: string;
		definitions: string;
		members: string;
	};

	type Props = {
		activeTab: string;
		labels: FlowTabLabels;
		onSelectTab: (value: string) => void;
	};

	let { activeTab, labels, onSelectTab }: Props = $props();

	let tabs = $derived(buildFlowTaskTabs());
</script>

<UnderlineTabs.Root value={activeTab} onValueChange={onSelectTab} data-flow-active-tab={activeTab}>
	<UnderlineTabs.List>
		{#each tabs as tab (tab)}
			<UnderlineTabs.Trigger value={tab}>{labels[tab]}</UnderlineTabs.Trigger>
		{/each}
	</UnderlineTabs.List>
</UnderlineTabs.Root>
