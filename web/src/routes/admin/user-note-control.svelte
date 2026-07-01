<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import { Textarea } from '$lib/components/ui/textarea';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import StickyNoteIcon from '@lucide/svelte/icons/sticky-note';
	import { Popover } from 'bits-ui';
	import type { AdminPageText } from './admin-types';

	type UserNoteControlProps = {
		note: string | undefined;
		text: AdminPageText['users'];
		isSaving: boolean;
		onSave: (note: string) => Promise<boolean>;
	};

	let { note, text, isSaving, onSave }: UserNoteControlProps = $props();

	let isOpen = $state(false);
	let draft = $state('');

	function noteText(): string {
		return note?.trim() ?? '';
	}

	function handleOpenChange(next: boolean) {
		isOpen = next;
		if (next) draft = note ?? '';
	}

	async function save() {
		if (await onSave(draft)) isOpen = false;
	}
</script>

<Popover.Root bind:open={isOpen} onOpenChange={handleOpenChange}>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button
				{...props}
				type="button"
				variant="ghost"
				size="icon-sm"
				class="relative text-foreground hover:text-primary"
				aria-label={text.noteEdit}
				title={noteText() || text.noteEdit}
			>
				<StickyNoteIcon class="size-4" />
				{#if noteText()}
					<span class="absolute right-1.5 top-1.5 size-1.5 rounded-full bg-primary"></span>
				{/if}
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			side="top"
			sideOffset={8}
			class="z-50 w-[min(20rem,calc(100vw-2rem))] rounded-md border bg-popover p-3 text-popover-foreground shadow-lg outline-none"
		>
			<div class="grid gap-3">
				<label class="grid gap-1.5">
					<Label>{text.note}</Label>
					<Textarea bind:value={draft} class="min-h-28 resize-y text-sm" placeholder={text.notePlaceholder} />
				</label>
				<div class="flex justify-end gap-2">
					<Popover.Close>
						{#snippet child({ props })}
							<Button {...props} type="button" variant="outline" size="sm" disabled={isSaving}>{text.cancel}</Button>
						{/snippet}
					</Popover.Close>
					<Button type="button" size="sm" disabled={isSaving} onclick={save}>
						{#if isSaving}
							<RefreshCwIcon class="size-4 animate-spin" />
						{/if}
						{text.save}
					</Button>
				</div>
			</div>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
