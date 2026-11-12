<script lang="ts">
	import ColorMarker from '$lib/components/color-marker.svelte';
	import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
	import CompanyAvatar from '$lib/components/company-avatar.svelte';
	import CountBadge from '$lib/components/count-badge.svelte';
	import DefinitionBadge from '$lib/components/definition-badge.svelte';
	import GradientAvatar from '$lib/components/gradient-avatar.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import PersonChip from '$lib/components/person-chip.svelte';
	import RefreshButton from '$lib/components/refresh-button.svelte';
	import TaskStatusBadge from '$lib/components/task-status-badge.svelte';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import UnreadDot from '$lib/components/unread-dot.svelte';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PaperclipIcon from '@lucide/svelte/icons/paperclip';
	import type { Pattern } from '$lib/avatar-gradient/engine';
	import { taskStatus } from '../task/task-status';
	import { taskStatusLabel } from '../task/task-options';
	import { taskText } from '../task/text';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);
	const channel = createPageText(channelText);
	const task = createPageText(taskText);

	const people = [
		{ name: '이샘플', seed: 'design-one' },
		{ name: '박예시', seed: 'design-two' },
		{ name: '최견본', seed: 'design-three' },
		{ name: 'Sample Park', seed: 'design-four' },
		{ name: 'Example Choi', seed: 'design-five' }
	];
	const definitionColors = ['#2563eb', '#16a34a', '#d97706', '#dc2626', '#7c3aed', ''];
	const gradientPatterns: Pattern[] = ['mesh', 'dither'];
	const refreshBriefly = () => new Promise((resolve) => setTimeout(resolve, 1200));
</script>

<DesignSection {pageID} title={text.peopleAndCompanySection}>
	<DesignFrame id="person-avatar" name="PersonAvatar" source="person-avatar.svelte">
		{#snippet children()}
			<div class="flex items-end gap-3">
				<PersonAvatar name="이샘플" seed="design-one" class="size-6" />
				<PersonAvatar name="이샘플" seed="design-one" class="size-8" isOnline presenceLabel={channel.presenceOnline} />
				<PersonAvatar name="박예시" seed="design-two" class="size-10" isOnline={false} presenceLabel={channel.presenceOffline} />
				<PersonAvatar name="최견본" seed="design-three" class="size-14" />
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="person-avatar-stack" name="PersonAvatarStack" source="person-avatar-stack.svelte">
		{#snippet children()}
			<PersonAvatarStack people={people.slice(0, 2)} />
			<PersonAvatarStack {people} />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="person-chip" name="PersonChip" source="person-chip.svelte">
		{#snippet children()}
			<div class="flex flex-wrap gap-2">
				{#each people.slice(0, 3) as person (person.seed)}
					<PersonChip name={person.name} seed={person.seed} />
				{/each}
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="company-avatar" name="CompanyAvatar · GradientAvatar" source="company-avatar.svelte">
		{#snippet children()}
			<div class="flex items-center gap-3">
				<CompanyAvatar name="샘플컴퍼니" class="size-10" />
				<CompanyAvatar name="Example Inc" class="size-10" />
				{#each gradientPatterns as pattern (pattern)}
					<span class="relative size-10 overflow-hidden rounded-full"><GradientAvatar seed="design-gradient" {pattern} /></span>
				{/each}
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

<DesignSection {pageID} title={text.badgesSection}>
	<DesignFrame id="task-status-badge" name="TaskStatusBadge" source="task-status-badge.svelte" class="w-96">
		{#snippet children()}
			<div class="flex flex-wrap gap-2">
				{#each Object.values(taskStatus) as status (status)}
					<TaskStatusBadge {status} label={taskStatusLabel(task, status)} />
				{/each}
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="count-badge" name="CountBadge · UnreadDot" source="count-badge.svelte">
		{#snippet children()}
			<div class="flex flex-wrap items-center gap-3">
				<CountBadge count={3} />
				<CountBadge count={12} variant="secondary" label={text.sample.attachments}>
					{#snippet icon()}<PaperclipIcon />{/snippet}
				</CountBadge>
				<UnreadDot label={text.sample.unread} />
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="definition-badge" name="DefinitionBadge · ColoredOutlineBadge · ColorMarker" source="definition-badge.svelte" class="w-96">
		{#snippet children()}
			<div class="flex flex-wrap gap-2">
				{#each definitionColors as color (color)}
					<DefinitionBadge label={color || text.sample.noColor} {color} />
				{/each}
			</div>
			<div class="flex flex-wrap gap-2">
				{#each definitionColors as color (color)}
					<ColoredOutlineBadge {color}>{text.sample.teamSales}</ColoredOutlineBadge>
				{/each}
			</div>
			<div class="flex flex-wrap items-center gap-3">
				{#each definitionColors as color (color)}
					<ColorMarker {color} />
				{/each}
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

<DesignSection {pageID} title={text.appActionsSection}>
	<DesignFrame id="tooltip-icon-button" name="TooltipIconButton · RefreshButton" source="tooltip-icon-button.svelte">
		{#snippet children()}
			<div class="flex items-center gap-2">
				<TooltipIconButton label={text.sample.attachments} variant="ghost" size="icon"><PaperclipIcon /></TooltipIconButton>
				<RefreshButton label={text.sample.refresh} refresh={refreshBriefly} variant="outline" size="icon" />
				<RefreshButton label={text.sample.refresh} refresh={refreshBriefly} variant="outline" showsLabel />
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>
