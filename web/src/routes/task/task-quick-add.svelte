<script lang="ts">
	import { onDestroy } from 'svelte';
	import AppFloatingActionButton from '$lib/components/app-floating-action-button.svelte';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { TaskQuickAddController } from './task-quick-add-controller.svelte';
	import type { TaskQuickCreateController } from './task-quick-create-controller.svelte';
	import TaskQuickAddPanel from './task-quick-add-panel.svelte';
	import { quickAddOverlayClass, quickAddPanelClass } from './task-quick-add-style';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskPageText = PageText<typeof taskText>;

	type Props = {
		quickTask: TaskQuickCreateController;
		hasMembers: boolean;
		text: TaskPageText['task'];
	};

	let { quickTask, hasMembers, text }: Props = $props();

	const quickAddPanel = new TaskQuickAddController();

	let launcherLabel = $derived(quickAddPanel.isOpen ? text.quickAddClose : text.quickAddTitle);
	let panelClass = $derived(quickAddPanelClass(quickAddPanel.isPanelVisible));
	let overlayClass = $derived(quickAddOverlayClass(quickAddPanel.isPanelVisible));

	function toggleQuickAddPanel(): void {
		quickAddPanel.toggle(hasMembers);
	}

	function closeQuickAddPanel(): void {
		quickAddPanel.close(true);
	}

	onDestroy(() => {
		quickAddPanel.destroy();
	});
</script>

<svelte:window onkeydown={quickAddPanel.handleKeydown} />

{#if quickAddPanel.isPanelMounted}
	<button
		type="button"
		class={overlayClass}
		aria-label={text.quickAddBackdropClose}
		onclick={closeQuickAddPanel}
	></button>
	<div
		id="task-quick-add-panel"
		role="dialog"
		aria-modal="true"
		aria-labelledby="task-quick-add-title"
		aria-describedby="task-quick-add-description"
		class={panelClass}
	>
		<TaskQuickAddPanel
			{quickTask}
			{hasMembers}
			{text}
			bind:textareaElement={quickAddPanel.textareaElement}
			bind:submitButtonElement={quickAddPanel.submitButtonElement}
		/>
	</div>
{/if}

<AppFloatingActionButton
	bind:ref={quickAddPanel.launcherElement}
	label={launcherLabel}
	data-task-quick-add-launcher
	aria-controls="task-quick-add-panel"
	aria-expanded={quickAddPanel.isOpen}
	disabled={!hasMembers && !quickAddPanel.isOpen}
	onclick={toggleQuickAddPanel}
>
	{#if quickAddPanel.isOpen}
		<XIcon />
	{:else}
		<PlusIcon />
	{/if}
</AppFloatingActionButton>

<style>
	:global(body:has([data-slot='sheet-content'][data-state='open']) [data-task-quick-add-launcher][aria-expanded='false']),
	:global(body:has([data-slot='sheet-content'][data-open]) [data-task-quick-add-launcher][aria-expanded='false']) {
		pointer-events: none;
		opacity: 0;
		transform: translateY(0.5rem) scale(0.95);
	}
</style>
