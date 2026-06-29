<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import type { DraftPopoverState } from './calendar-draft-popover-state';
	import type { DraftPopoverText } from './calendar-draft-popover-text';

	type Props = {
		popover: DraftPopoverState;
		text: DraftPopoverText;
		updatePopover: (changes: Partial<DraftPopoverState>) => void;
		savePopover: () => void;
		cancelPopover: () => void;
	};

	let { popover, text, updatePopover, savePopover, cancelPopover }: Props = $props();

	let titleInputElement: HTMLInputElement | null = null;
	let focusedTitleEventID = '';
	let pendingTitleFocusEventID = '';
	let titleFocusFrame: number | null = null;

	function inputValue(event: Event): string {
		return event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : '';
	}

	function focusTitleInputForCreatePopover(): void {
		const eventID = popover.eventID;
		pendingTitleFocusEventID = eventID;
		void tick().then(() => {
			if (pendingTitleFocusEventID !== eventID) return;
			if (popover.eventID !== eventID || popover.mode !== 'create') return;
			if (titleFocusFrame !== null) cancelAnimationFrame(titleFocusFrame);
			titleFocusFrame = requestAnimationFrame(() => {
				titleFocusFrame = null;
				if (pendingTitleFocusEventID !== eventID) return;
				if (popover.eventID !== eventID || popover.mode !== 'create') return;
				titleInputElement?.focus({ preventScroll: true });
				titleInputElement?.select();
				focusedTitleEventID = eventID;
			});
		});
	}

	$effect(() => {
		popover.eventID;
		popover.mode;
		popover.position.isReady;
		if (popover.mode !== 'create') {
			focusedTitleEventID = '';
			return;
		}
		if (!popover.position.isReady) return;
		if (focusedTitleEventID === popover.eventID) return;
		focusTitleInputForCreatePopover();
	});

	onDestroy(() => {
		pendingTitleFocusEventID = '';
		if (titleFocusFrame !== null) cancelAnimationFrame(titleFocusFrame);
	});
</script>

<label class="draft-popover-title-row">
	<input
		bind:this={titleInputElement}
		value={popover.title}
		aria-label={text.title}
		placeholder={text.title}
		autocomplete="off"
		oninput={(event) => updatePopover({ title: inputValue(event) })}
		onkeydown={(event) => {
			if (event.key === 'Enter') savePopover();
			if (event.key === 'Escape') cancelPopover();
		}}
	/>
	<button type="button" class="draft-popover-icon-button" aria-label={text.cancel} onclick={cancelPopover}>
		×
	</button>
</label>
