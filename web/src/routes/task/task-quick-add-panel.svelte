<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import TaskQuickAddOutcomes from './task-quick-add-outcomes.svelte';
	import type { TaskQuickCreateController } from './task-quick-create-controller.svelte';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskPageText = PageText<typeof taskText>;

	type Props = {
		quickTask: TaskQuickCreateController;
		hasMembers: boolean;
		text: TaskPageText['task'];
		textareaElement: HTMLTextAreaElement | null;
		submitButtonElement: HTMLElement | null;
	};

	let {
		quickTask,
		hasMembers,
		text,
		textareaElement = $bindable(null),
		submitButtonElement = $bindable(null)
	}: Props = $props();

	let canSubmit = $derived(hasMembers && !quickTask.isAdding && quickTask.note.trim() !== '');

	function submitOnModifierEnter(event: KeyboardEvent): void {
		if (event.key !== 'Enter' || !(event.metaKey || event.ctrlKey) || !canSubmit) return;
		event.preventDefault();
		void quickTask.addFromNote();
	}
</script>

<header class="space-y-1 px-5 pt-5">
	<h2 id="task-quick-add-title" class="font-semibold leading-none">{text.quickAddTitle}</h2>
	<p id="task-quick-add-description" class="text-sm text-muted-foreground">{text.quickAddHint}</p>
</header>
<div class="flex min-h-0 flex-1 flex-col gap-3 px-5 py-4">
	<Textarea
		bind:ref={textareaElement}
		class="max-h-40 min-h-28! resize-none sm:max-h-none sm:flex-1"
		placeholder={text.quickAddPlaceholder}
		bind:value={quickTask.note}
		onkeydown={submitOnModifierEnter}
	/>
	<Button
		bind:ref={submitButtonElement}
		class="w-full"
		onclick={quickTask.addFromNote}
		disabled={!canSubmit}
		aria-busy={quickTask.isAdding}
	>
		{#if quickTask.isAdding}<Spinner aria-hidden="true" />{/if}
		{quickTask.isAdding ? text.quickAddAdding : text.quickAddSubmit}
	</Button>
	{#if quickTask.failure}
		<div role="alert" class="rounded-lg border border-destructive/30 bg-destructive/10 p-2 text-sm text-destructive">
			<p>{text.quickAddFailure}</p>
			<p class="mt-1 break-words text-xs opacity-80">{quickTask.failure}</p>
		</div>
	{/if}
	{#if quickTask.outcomes}
		<TaskQuickAddOutcomes outcomes={quickTask.outcomes} {text} />
	{/if}
</div>
