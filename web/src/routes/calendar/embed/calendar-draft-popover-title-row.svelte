<script lang="ts">
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

	function inputValue(event: Event): string {
		return event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : '';
	}
</script>

<label class="draft-popover-title-row">
	<input
		data-draft-popover-initial-focus
		value={popover.title}
		aria-label={text.title}
		placeholder={text.title}
		autocomplete="off"
		oninput={(event) => updatePopover({ title: inputValue(event) })}
		onkeydown={(event) => {
			if (event.key === 'Enter') savePopover();
		}}
	/>
	<button type="button" class="draft-popover-icon-button" aria-label={text.cancel} onclick={cancelPopover}>
		×
	</button>
</label>
