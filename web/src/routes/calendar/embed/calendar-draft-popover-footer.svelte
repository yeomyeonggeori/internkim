<script lang="ts">
	import type { DraftPopoverState } from './calendar-draft-popover-state';
	import type { DraftPopoverText } from './calendar-draft-popover-text';

	type Props = {
		popover: DraftPopoverState;
		text: DraftPopoverText;
		isSaving: boolean;
		canSavePopover: boolean;
		savePopover: () => void;
		cancelPopover: () => void;
		deletePopover: () => void;
	};

	let { popover, text, isSaving, canSavePopover, savePopover, cancelPopover, deletePopover }: Props = $props();
</script>

<footer class="draft-popover-footer">
	{#if popover.mode === 'edit'}
		<button type="button" class="draft-popover-delete" disabled={isSaving} onclick={deletePopover}>
			{text.delete}
		</button>
	{/if}
	<button type="button" class="draft-popover-cancel" disabled={isSaving} onclick={cancelPopover}>
		{text.cancel}
	</button>
	<button type="button" class="draft-popover-complete" disabled={isSaving || !canSavePopover} onclick={savePopover}>
		{text.complete}
	</button>
</footer>
