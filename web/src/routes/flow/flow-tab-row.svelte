<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { cn } from '$lib/utils';

	type FlowTabMember = {
		id: string;
		name: string;
		email: string;
	};

	type FlowTabLabels = {
		report: string;
		tasks: string;
		definitions: string;
		members: string;
	};

	type Props = {
		activeTab: string;
		labels: FlowTabLabels;
		members: FlowTabMember[];
		canEditDefinitions: boolean;
		onSelectTab: (value: string) => void;
	};

	let { activeTab, labels, members, canEditDefinitions, onSelectTab }: Props = $props();

	function isActive(value: string): boolean {
		return activeTab === value;
	}

	function selectTab(value: string, disabled = false): void {
		if (disabled) return;
		onSelectTab(value);
	}
</script>

<div class="flex w-full min-w-0 items-center gap-1 overflow-x-auto border-b">
	<button
		type="button"
		class={cn(
			'relative h-9 whitespace-nowrap px-3 text-sm font-medium transition-colors',
			isActive('report') ? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary' : 'text-muted-foreground hover:text-foreground'
		)}
		onclick={() => selectTab('report')}
	>
		{labels.report}
	</button>
	<button
		type="button"
		class={cn(
			'relative h-9 whitespace-nowrap px-3 text-sm font-medium transition-colors',
			isActive('tasks') ? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary' : 'text-muted-foreground hover:text-foreground'
		)}
		onclick={() => selectTab('tasks')}
	>
		{labels.tasks}
	</button>
	<button
		type="button"
		class={cn(
			'relative h-9 whitespace-nowrap px-3 text-sm font-medium transition-colors',
			isActive('definitions') ? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary' : 'text-muted-foreground hover:text-foreground',
			!canEditDefinitions && 'cursor-not-allowed opacity-50 hover:text-muted-foreground'
		)}
		disabled={!canEditDefinitions}
		onclick={() => selectTab('definitions', !canEditDefinitions)}
	>
		{labels.definitions}
	</button>
	<button
		type="button"
		class={cn(
			'relative h-9 whitespace-nowrap px-3 text-sm font-medium transition-colors',
			isActive('members') ? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary' : 'text-muted-foreground hover:text-foreground'
		)}
		onclick={() => selectTab('members')}
	>
		{labels.members}
	</button>
	{#if members.length > 0}
		<div class="mx-2 h-5 w-px shrink-0 bg-border"></div>
	{/if}
	{#each members as member}
		<button
			type="button"
			class={cn(
				'relative inline-flex h-9 items-center gap-2 whitespace-nowrap px-3 text-sm font-medium transition-colors',
				isActive(`member:${member.id}`)
					? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary'
					: 'text-muted-foreground hover:text-foreground'
			)}
			onclick={() => selectTab(`member:${member.id}`)}
		>
			<PersonAvatar name={member.name} email={member.email} class="size-5" />
			{member.name}
		</button>
	{/each}
</div>
