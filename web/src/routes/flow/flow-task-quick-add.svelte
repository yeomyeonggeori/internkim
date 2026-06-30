<script lang="ts">
	import { onDestroy } from 'svelte';
	import { FlowTaskQuickAddController } from './flow-task-quick-add-controller.svelte';
	import FlowTaskQuickAddLauncher from './flow-task-quick-add-launcher.svelte';
	import FlowTaskQuickAddPanel from './flow-task-quick-add-panel.svelte';
	import {
		quickAddLauncherClass,
		quickAddLauncherContentClass,
		quickAddOverlayClass,
		quickAddPanelClass
	} from './flow-task-quick-add-style';
	import type { FlowQuickTaskCreateResult } from './flow-types';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		quickTaskText: string;
		taskErrorMessage: string;
		quickTaskDuplicateMessage: string;
		isCreatingQuickTask: boolean;
		hasMembers: boolean;
		text: FlowPageText['task'];
		createQuickTask: () => Promise<FlowQuickTaskCreateResult>;
		confirmQuickTaskDuplicate: () => Promise<FlowQuickTaskCreateResult>;
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

	const quickAddPanel = new FlowTaskQuickAddController();

	let launcherClass = $derived(quickAddLauncherClass(quickAddPanel.isOpen));
	let launcherLabel = $derived(quickAddPanel.isOpen ? text.quickAddClose : text.quickAdd);
	let closedLauncherContentClass = $derived(quickAddLauncherContentClass(!quickAddPanel.isOpen));
	let openLauncherContentClass = $derived(quickAddLauncherContentClass(quickAddPanel.isOpen));
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

	async function submitQuickTaskWith(createTask: () => Promise<FlowQuickTaskCreateResult>): Promise<void> {
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
		id="flow-ai-quick-add-panel"
		role="dialog"
		aria-modal="true"
		aria-labelledby="flow-ai-quick-add-title"
		aria-describedby="flow-ai-quick-add-description"
		class={panelClass}
	>
		<FlowTaskQuickAddPanel
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

<FlowTaskQuickAddLauncher
	isOpen={quickAddPanel.isOpen}
	{hasMembers}
	{launcherClass}
	closedContentClass={closedLauncherContentClass}
	openContentClass={openLauncherContentClass}
	{launcherLabel}
	quickAddLabel={text.quickAdd}
	bind:launcherElement={quickAddPanel.launcherElement}
	{toggleQuickAddPanel}
/>

<style>
	:global(body:has([data-slot='sheet-content'][data-state='open']) [data-flow-quick-add-launcher][aria-expanded='false']),
	:global(body:has([data-slot='sheet-content'][data-open]) [data-flow-quick-add-launcher][aria-expanded='false']) {
		pointer-events: none;
		opacity: 0;
		transform: translateY(0.5rem) scale(0.95);
	}
</style>
