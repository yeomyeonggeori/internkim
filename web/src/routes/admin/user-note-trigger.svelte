<!-- 사용자 메모 팝업을 여는 아이콘 버튼을 렌더링한다. -->
<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import StickyNoteIcon from '@lucide/svelte/icons/sticky-note';

	type UserNoteTriggerProps = {
		note: string | undefined;
		isOpen: boolean;
		label: string;
		fallbackTitle: string;
		onOpen: (event: MouseEvent) => void;
	};

	let { note, isOpen, label, fallbackTitle, onOpen }: UserNoteTriggerProps = $props();

	function noteText(): string {
		return note?.trim() ?? '';
	}
</script>

<Button
	type="button"
	variant="ghost"
	size="icon-sm"
	class="relative text-foreground hover:text-primary"
	aria-label={label}
	aria-expanded={isOpen}
	title={noteText() || fallbackTitle}
	onclick={onOpen}
>
	<StickyNoteIcon class="size-4" />
	{#if noteText()}
		<span class="absolute right-1.5 top-1.5 size-1.5 rounded-full bg-primary"></span>
	{/if}
</Button>
