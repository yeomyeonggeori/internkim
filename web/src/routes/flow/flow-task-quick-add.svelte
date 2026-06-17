<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import XIcon from '@lucide/svelte/icons/x';
	import { onDestroy, tick } from 'svelte';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		quickTaskText: string;
		taskErrorMessage: string;
		quickTaskDuplicateMessage: string;
		isCreatingQuickTask: boolean;
		hasMembers: boolean;
		text: FlowPageText['task'];
		createQuickTask: () => Promise<void>;
		confirmQuickTaskDuplicate: () => Promise<void>;
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

	let isOpen = $state(false);
	let isPanelMounted = $state(false);
	let isPanelVisible = $state(false);
	let closeAnimationTimeout = $state<ReturnType<typeof setTimeout> | null>(null);
	let quickAddTextareaElement = $state<HTMLTextAreaElement | null>(null);
	let quickAddSubmitButtonElement = $state<HTMLElement | null>(null);
	let quickAddLauncherElement = $state<HTMLElement | null>(null);

	const closedLauncherClass =
		'fixed bottom-5 right-5 z-[60] h-10 w-36 rounded-full px-3 text-sm shadow-lg transition-all duration-[400ms] md:bottom-6 md:right-6 md:px-4';
	const openLauncherClass =
		'fixed bottom-5 right-5 z-[60] size-12 rounded-full bg-destructive px-0 text-destructive-foreground shadow-xl transition-all duration-[400ms] hover:bg-destructive/90 md:bottom-6 md:right-6';
	const visibleLauncherContentClass =
		'absolute inset-0 flex items-center justify-center gap-1.5 opacity-100 scale-100 transition-all duration-[400ms]';
	const hiddenLauncherContentClass =
		'absolute inset-0 flex items-center justify-center gap-1.5 opacity-0 scale-75 transition-all duration-[400ms]';
	const panelBaseClass =
		'fixed bottom-[5.75rem] right-5 z-[55] flex h-[min(500px,calc(100vh-7rem))] w-[calc(100vw-2rem)] max-w-[400px] flex-col overflow-hidden rounded-2xl bg-popover text-popover-foreground shadow-2xl ring-1 ring-foreground/10 transition-all duration-[400ms] md:bottom-[6.25rem] md:right-6';
	const visiblePanelClass = 'translate-y-0 scale-100 opacity-100';
	const hiddenPanelClass = 'pointer-events-none translate-y-3 scale-95 opacity-0';

	let launcherClass = $derived(isOpen ? openLauncherClass : closedLauncherClass);
	let launcherLabel = $derived(isOpen ? text.quickAddClose : text.quickAdd);
	let closedLauncherContentClass = $derived(isOpen ? hiddenLauncherContentClass : visibleLauncherContentClass);
	let openLauncherContentClass = $derived(isOpen ? visibleLauncherContentClass : hiddenLauncherContentClass);
	let panelClass = $derived(`${panelBaseClass} ${isPanelVisible ? visiblePanelClass : hiddenPanelClass}`);
	let overlayClass = $derived(
		`fixed inset-0 z-50 cursor-default bg-black/10 transition-opacity duration-[400ms] supports-backdrop-filter:backdrop-blur-xs ${isPanelVisible ? 'opacity-100' : 'pointer-events-none opacity-0'}`
	);

	function toggleQuickAddPanel(): void {
		if (isOpen) {
			beginCloseQuickAddPanel(false);
			return;
		}
		openQuickAddPanel();
	}

	function closeQuickAddPanel(): void {
		beginCloseQuickAddPanel(true);
	}

	function handleQuickAddKeydown(event: KeyboardEvent): void {
		if (!isPanelMounted) return;
		if (event.key === 'Escape') {
			event.stopPropagation();
			closeQuickAddPanel();
			return;
		}
		if (!isOpen || event.key !== 'Tab') return;
		trapQuickAddFocus(event);
	}

	function openQuickAddPanel(): void {
		if (!hasMembers) return;
		if (closeAnimationTimeout) clearTimeout(closeAnimationTimeout);
		closeAnimationTimeout = null;
		isPanelMounted = true;
		isOpen = true;
		void tick().then(() => {
			isPanelVisible = true;
			quickAddTextareaElement?.focus();
		});
	}

	function beginCloseQuickAddPanel(restoreFocus: boolean): void {
		if (!isPanelMounted) return;
		if (closeAnimationTimeout) clearTimeout(closeAnimationTimeout);
		isOpen = false;
		isPanelVisible = false;
		closeAnimationTimeout = setTimeout(() => {
			isPanelMounted = false;
			closeAnimationTimeout = null;
		}, 400);
		if (restoreFocus) {
			void tick().then(() => quickAddLauncherElement?.focus());
		}
	}

	function trapQuickAddFocus(event: KeyboardEvent): void {
		const focusableElements = [quickAddTextareaElement, quickAddSubmitButtonElement, quickAddLauncherElement]
			.filter(isFocusableElement);
		if (focusableElements.length === 0) return;
		const currentIndex = document.activeElement instanceof HTMLElement
			? focusableElements.indexOf(document.activeElement)
			: -1;
		const nextIndex = event.shiftKey
			? (currentIndex <= 0 ? focusableElements.length - 1 : currentIndex - 1)
			: (currentIndex === -1 || currentIndex >= focusableElements.length - 1 ? 0 : currentIndex + 1);
		event.preventDefault();
		focusableElements[nextIndex]?.focus();
	}

	function isFocusableElement(element: HTMLElement | null): element is HTMLElement {
		if (!element) return false;
		if (element instanceof HTMLButtonElement || element instanceof HTMLTextAreaElement) return !element.disabled;
		return true;
	}

	onDestroy(() => {
		if (closeAnimationTimeout) clearTimeout(closeAnimationTimeout);
	});
</script>

<svelte:window onkeydown={handleQuickAddKeydown} />

{#if isPanelMounted}
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
		<header class="space-y-1 px-5 pt-5">
			<h2 id="flow-ai-quick-add-title" class="font-semibold leading-none">{text.quickAdd}</h2>
			<p id="flow-ai-quick-add-description" class="text-sm text-muted-foreground">{text.quickAddHint}</p>
		</header>
		<div class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4">
			<Textarea
				bind:ref={quickAddTextareaElement}
				class="min-h-0 flex-1 resize-none"
				placeholder={text.quickAddPlaceholder}
				bind:value={quickTaskText}
			/>
			<Button
				bind:ref={quickAddSubmitButtonElement}
				class="w-full"
				onclick={createQuickTask}
				disabled={isCreatingQuickTask || !quickTaskText.trim() || !hasMembers}
			>
				<SparklesIcon />
				{isCreatingQuickTask ? text.saving : text.quickAdd}
			</Button>
			{#if taskErrorMessage}
				<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-2 text-sm text-destructive">{taskErrorMessage}</p>
			{/if}
			{#if quickTaskDuplicateMessage}
				<div class="flex flex-col gap-2 rounded-lg border border-amber-200 bg-amber-50 p-2 text-sm text-amber-900">
					<p>{quickTaskDuplicateMessage}</p>
					<Button variant="outline" size="sm" onclick={confirmQuickTaskDuplicate} disabled={isCreatingQuickTask}>
						{text.quickAddDuplicateAction}
					</Button>
				</div>
			{/if}
		</div>
	</div>
{/if}

<Button
	type="button"
	size={isOpen ? 'icon-lg' : 'lg'}
	bind:ref={quickAddLauncherElement}
	class={`${launcherClass} overflow-hidden`}
	disabled={!hasMembers && !isOpen}
	aria-label={launcherLabel}
	aria-controls="flow-ai-quick-add-panel"
	aria-expanded={isOpen}
	onclick={toggleQuickAddPanel}
>
	<span data-flow-quick-add-closed-content class={closedLauncherContentClass} aria-hidden={isOpen}>
		<SparklesIcon class="size-4 shrink-0" />
		<span>{text.quickAdd}</span>
	</span>
	<span data-flow-quick-add-open-content class={openLauncherContentClass} aria-hidden={!isOpen}>
		<XIcon class="size-6" />
	</span>
</Button>
