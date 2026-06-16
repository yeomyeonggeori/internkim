<script lang="ts">
	import { cn } from '$lib/utils';
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

	function isActive(value: string): boolean {
		return activeTab === value;
	}

	function selectTab(value: string, disabled = false): void {
		if (disabled) return;
		onSelectTab(value);
	}
</script>

<div class="flex w-full min-w-0 items-center gap-1 overflow-x-auto border-b" data-flow-active-tab={activeTab}>
	{#each tabs as tab}
		<button
			type="button"
			class={cn(
				'relative h-9 whitespace-nowrap px-3 text-sm font-medium transition-colors',
				isActive(tab) ? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary' : 'text-muted-foreground hover:text-foreground'
			)}
			onclick={() => selectTab(tab)}
		>
			{labels[tab]}
		</button>
	{/each}
</div>
