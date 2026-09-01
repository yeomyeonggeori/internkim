<script lang="ts">
	import { onDestroy } from 'svelte';
	import AppFloatingActionButton from '$lib/components/app-floating-action-button.svelte';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { TaskQuickAddController } from './task-quick-add-controller.svelte';
	import TaskQuickAddPanel from './task-quick-add-panel.svelte';
	import { quickAddOverlayClass, quickAddPanelClass } from './task-quick-add-style';
	import type { TaskQuickTaskCreateResult } from './task-types';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskPageText = PageText<typeof taskText>;

	type Props = {
		quickTaskText: string;
		taskErrorMessage: string;
		quickTaskDuplicateMessage: string;
		isCreatingQuickTask: boolean;
		hasMembers: boolean;
		text: TaskPageText['task'];
		createQuickTask: () => Promise<TaskQuickTaskCreateResult>;
		confirmQuickTaskDuplicate: () => Promise<TaskQuickTaskCreateResult>;
	};

	let {
		quickTaskText = $bindable(''),
		taskErrorMessage,
		quickTaskDuplicateMessage,
		isCreatingQuickTask,
		hasMembers,
		text,
		createQuickTask,
		confirmQuickTaskDuplicate
	}: Props = $props();

	const quickAddPanel = new TaskQuickAddController();

	let launcherLabel = $derived(quickAddPanel.isOpen ? text.quickAddClose : text.quickAdd);
	let panelClass = $derived(quickAddPanelClass(quickAddPanel.isPanelVisible));
	let overlayClass = $derived(quickAddOverlayClass(quickAddPanel.isPanelVisible));

	function toggleQuickAddPanel(): void {
		quickAddPanel.toggle(hasMembers);
	}

	function closeQuickAddPanel(): void {
		quickAddPanel.close(true);
	}

	async function submitQuickTask(): Promise<void> {
		await submitQuickTaskWith(createQuickTask);
	}

	async function confirmDuplicateQuickTask(): Promise<void> {
		await submitQuickTaskWith(confirmQuickTaskDuplicate);
	}

	async function submitQuickTaskWith(createTask: () => Promise<TaskQuickTaskCreateResult>): Promise<void> {
		await quickAddPanel.submitWith(createTask);
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
		id="task-ai-quick-add-panel"
		role="dialog"
		aria-modal="true"
		aria-labelledby="task-ai-quick-add-title"
		aria-describedby="task-ai-quick-add-description"
		class={panelClass}
	>
		<TaskQuickAddPanel
			bind:quickTaskText
			taskErrorMessage={taskErrorMessage}
			quickTaskDuplicateMessage={quickTaskDuplicateMessage}
			isCreatingQuickTask={isCreatingQuickTask}
			hasMembers={hasMembers}
			text={text}
			bind:textareaElement={quickAddPanel.textareaElement}
			bind:submitButtonElement={quickAddPanel.submitButtonElement}
			submitQuickTask={submitQuickTask}
			confirmQuickTaskDuplicate={confirmDuplicateQuickTask}
		/>
	</div>
{/if}

<AppFloatingActionButton
	bind:ref={quickAddPanel.launcherElement}
	label={launcherLabel}
	data-task-quick-add-launcher
	aria-controls="task-ai-quick-add-panel"
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
