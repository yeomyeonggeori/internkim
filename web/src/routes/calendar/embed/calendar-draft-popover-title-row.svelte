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

	function inputValue(event: Event): string {
		return event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : '';
	}

	function focusTitleInputForCreatePopover(): void {
		const eventID = popover.eventID;
		pendingTitleFocusEventID = eventID;
		void tick().then(() => {
			if (pendingTitleFocusEventID !== eventID) return;
			if (popover.eventID !== eventID || popover.mode !== 'create') return;
			titleInputElement?.focus({ preventScroll: true });
			titleInputElement?.select();
		});
	}

	$effect(() => {
		popover.eventID;
		popover.mode;
		if (popover.mode !== 'create') {
			focusedTitleEventID = '';
			return;
		}
		if (focusedTitleEventID === popover.eventID) return;
		focusedTitleEventID = popover.eventID;
		focusTitleInputForCreatePopover();
	});

	onDestroy(() => {
		pendingTitleFocusEventID = '';
	});
</script>

<label class="draft-popover-title-row">
	<span class="draft-popover-color-dot" aria-hidden="true"></span>
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
