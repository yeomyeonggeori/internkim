<!-- admin 사용자 그룹 관리 패널을 렌더링한다. -->
<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import XIcon from '@lucide/svelte/icons/x';
	import type { AdminPageText, CircleRecord } from './admin-types';

	type UserGroupsPanelProps = {
		text: AdminPageText['users'];
		circles: CircleRecord[];
		circleID: string;
		circleName: string;
		isMattermostManaged: boolean;
		isSaving: boolean;
		onSave: () => void;
		onRemove: (circleID: string) => void;
	};

	let {
		text,
		circles,
		circleID = $bindable(),
		circleName = $bindable(),
		isMattermostManaged = $bindable(),
		isSaving,
		onSave,
		onRemove
	}: UserGroupsPanelProps = $props();
</script>

<Card.Root>
	<Card.Header class="gap-1">
		<Card.Title class="text-sm">{text.groupTitle}</Card.Title>
		<Card.Description>{text.groupDescription}</Card.Description>
	</Card.Header>
	<Card.Content>
		<form
			class="grid gap-3 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end"
			onsubmit={(event) => {
				event.preventDefault();
				onSave();
			}}
		>
			<label class="grid gap-1.5">
				<Label>{text.groupID}</Label>
				<Input bind:value={circleID} placeholder={text.groupIDPlaceholder} autocomplete="off" />
			</label>
			<label class="grid gap-1.5">
				<Label>{text.groupName}</Label>
				<Input bind:value={circleName} placeholder={text.groupNamePlaceholder} autocomplete="off" />
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={isMattermostManaged} />
				{text.mattermostManaged}
			</label>
			<Button type="submit" disabled={isSaving || !circleID.trim()}>{text.addGroup}</Button>
		</form>
		<div class="mt-3 flex flex-wrap gap-2">
			{#each circles as circle (circle.circleID)}
				<Badge variant="outline" class="gap-2">
					{circle.displayName || circle.circleID}
					{#if circle.circleID !== 'staff'}
						<button type="button" class="text-muted-foreground hover:text-destructive" onclick={() => onRemove(circle.circleID)}>
							<XIcon class="size-3" />
						</button>
					{/if}
				</Badge>
			{/each}
		</div>
	</Card.Content>
</Card.Root>
