<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import XIcon from '@lucide/svelte/icons/x';

	type Props = {
		isOpen: boolean;
		hasMembers: boolean;
		launcherClass: string;
		closedContentClass: string;
		openContentClass: string;
		launcherLabel: string;
		quickAddLabel: string;
		launcherElement: HTMLElement | null;
		toggleQuickAddPanel: () => void;
	};

	let {
		isOpen,
		hasMembers,
		launcherClass,
		closedContentClass,
		openContentClass,
		launcherLabel,
		quickAddLabel,
		launcherElement = $bindable(null),
		toggleQuickAddPanel
	}: Props = $props();
</script>

<Button
	type="button"
	size={isOpen ? 'icon-lg' : 'lg'}
	bind:ref={launcherElement}
	class={`${launcherClass} overflow-hidden`}
	data-flow-quick-add-launcher
	disabled={!hasMembers && !isOpen}
	aria-label={launcherLabel}
	aria-controls="flow-ai-quick-add-panel"
	aria-expanded={isOpen}
	onclick={toggleQuickAddPanel}
>
	<span data-flow-quick-add-closed-content class={closedContentClass} aria-hidden={isOpen}>
		<SparklesIcon class="size-4 shrink-0" />
		<span>{quickAddLabel}</span>
	</span>
	<span data-flow-quick-add-open-content class={openContentClass} aria-hidden={!isOpen}>
		<XIcon class="size-6" />
	</span>
</Button>
