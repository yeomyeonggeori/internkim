import { tick } from 'svelte';
import type { FlowQuickTaskCreateResult } from './flow-types';

const closeAnimationDurationMilliseconds = 400;

export class FlowTaskQuickAddController {
	isOpen = $state(false);
	isPanelMounted = $state(false);
	isPanelVisible = $state(false);
	textareaElement = $state<HTMLTextAreaElement | null>(null);
	submitButtonElement = $state<HTMLElement | null>(null);
	launcherElement = $state<HTMLButtonElement | null>(null);

	private closeAnimationTimeout = $state<ReturnType<typeof setTimeout> | null>(null);

	toggle = (hasMembers: boolean): void => {
		if (this.isOpen) {
			this.close(false);
			return;
		}
		this.open(hasMembers);
	};

	open = (hasMembers: boolean): void => {
		if (!hasMembers) return;
		this.clearCloseAnimationTimeout();
		this.isPanelMounted = true;
		this.isOpen = true;
		void tick().then(() => {
			this.isPanelVisible = true;
			this.textareaElement?.focus();
		});
	};

	close = (restoreFocus: boolean): void => {
		if (!this.isPanelMounted) return;
		this.clearCloseAnimationTimeout();
		this.isOpen = false;
		this.isPanelVisible = false;
		this.closeAnimationTimeout = setTimeout(() => {
			this.isPanelMounted = false;
			this.closeAnimationTimeout = null;
		}, closeAnimationDurationMilliseconds);
		if (restoreFocus) {
			void tick().then(() => this.launcherElement?.focus());
		}
	};

	submitWith = async (createTask: () => Promise<FlowQuickTaskCreateResult>): Promise<void> => {
		const result = await createTask();
		if (result === 'created') this.close(true);
	};

	handleKeydown = (event: KeyboardEvent): void => {
		if (!this.isPanelMounted) return;
		if (event.key === 'Escape') {
			event.stopPropagation();
			this.close(true);
			return;
		}
		if (!this.isOpen || event.key !== 'Tab') return;
		this.trapFocus(event);
	};

	destroy = (): void => {
		this.clearCloseAnimationTimeout();
	};

	private trapFocus(event: KeyboardEvent): void {
		const focusableElements = [this.textareaElement, this.submitButtonElement, this.launcherElement]
			.filter(isFocusableElement);
		if (focusableElements.length === 0) return;
		const currentIndex = document.activeElement instanceof HTMLElement
			? focusableElements.indexOf(document.activeElement)
			: -1;
		const nextIndex = event.shiftKey
			? previousFocusableIndex(currentIndex, focusableElements.length)
			: nextFocusableIndex(currentIndex, focusableElements.length);
		event.preventDefault();
		focusableElements[nextIndex]?.focus();
	}

	private clearCloseAnimationTimeout(): void {
		if (!this.closeAnimationTimeout) return;
		clearTimeout(this.closeAnimationTimeout);
		this.closeAnimationTimeout = null;
	}
}

function previousFocusableIndex(currentIndex: number, focusableElementCount: number): number {
	if (currentIndex <= 0) return focusableElementCount - 1;
	return currentIndex - 1;
}

function nextFocusableIndex(currentIndex: number, focusableElementCount: number): number {
	if (currentIndex === -1 || currentIndex >= focusableElementCount - 1) return 0;
	return currentIndex + 1;
}

function isFocusableElement(element: HTMLElement | null): element is HTMLElement {
	if (!element) return false;
	if (element instanceof HTMLButtonElement || element instanceof HTMLTextAreaElement) return !element.disabled;
	return true;
}
