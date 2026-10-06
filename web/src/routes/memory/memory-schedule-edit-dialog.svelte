<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { scheduleKindLabel, type ScheduleEditDraft } from './memory-schedule-draft';
	import type { MemoryText } from './text';

	type Props = {
		text: MemoryText;
		isOpen: boolean;
		scheduleDraft: ScheduleEditDraft;
		isSavingSchedule: boolean;
		canSaveSchedule: boolean;
		saveSchedule: () => void;
	};

	let {
		text,
		isOpen = $bindable(),
		scheduleDraft = $bindable(),
		isSavingSchedule,
		canSaveSchedule,
		saveSchedule
	}: Props = $props();
</script>

<Dialog.Root bind:open={isOpen}>
	<Dialog.Content class="max-w-xl">
		<form
			class="grid gap-5"
			onsubmit={(event) => {
				event.preventDefault();
				saveSchedule();
			}}
		>
			<Dialog.Header>
				<Dialog.Title>{text.scheduleEditTitle}</Dialog.Title>
				<Dialog.Description>{text.scheduleEditDescription}</Dialog.Description>
			</Dialog.Header>

			<div class="grid gap-4">
				<div class="grid gap-2">
					<Label for="schedule-name">{text.scheduleNameLabel}</Label>
					<Input id="schedule-name" bind:value={scheduleDraft.name} />
				</div>

				<div class="grid gap-2">
					<Label>{text.scheduleKind}</Label>
					<Select.Root type="single" bind:value={scheduleDraft.kind}>
						<Select.Trigger class="w-full">{scheduleKindLabel(text, scheduleDraft.kind)}</Select.Trigger>
						<Select.Content><Select.Group>
							<Select.Item value="once" label={text.scheduleKindOnce}>{text.scheduleKindOnce}</Select.Item>
							<Select.Item value="interval" label={text.scheduleKindInterval}>{text.scheduleKindInterval}</Select.Item>
							<Select.Item value="cron" label={text.scheduleKindCron}>{text.scheduleKindCron}</Select.Item>
						</Select.Group></Select.Content>
					</Select.Root>
				</div>

				{#if scheduleDraft.kind === 'once'}
					<div class="grid gap-2">
						<Label for="schedule-run-at">{text.scheduleRunAtLabel}</Label>
						<Input id="schedule-run-at" type="datetime-local" bind:value={scheduleDraft.runAt} />
					</div>
				{:else if scheduleDraft.kind === 'interval'}
					<div class="grid gap-2">
						<Label for="schedule-interval-minute">{text.scheduleIntervalMinuteLabel}</Label>
						<Input id="schedule-interval-minute" type="number" min="1" step="1" bind:value={scheduleDraft.intervalMinute} />
					</div>
				{:else}
					<div class="grid gap-2">
						<Label for="schedule-cron-expression">{text.scheduleCronExpressionLabel}</Label>
						<Input id="schedule-cron-expression" bind:value={scheduleDraft.cronExpression} placeholder="0 9 * * *" />
					</div>
					<div class="grid gap-2">
						<Label for="schedule-time-zone">{text.scheduleTimeZoneLabel}</Label>
						<Input id="schedule-time-zone" bind:value={scheduleDraft.timeZone} placeholder="Asia/Seoul" />
					</div>
				{/if}

				{#if scheduleDraft.kind !== 'once'}
					<div class="grid gap-2">
						<Label>{text.scheduleRepeatPolicyLabel}</Label>
						<Select.Root type="single" bind:value={scheduleDraft.repeatPolicy}>
							<Select.Trigger class="w-full">{scheduleDraft.repeatPolicy === 'unbounded' ? text.scheduleRepeatUnbounded : text.scheduleRepeatFinite}</Select.Trigger>
							<Select.Content><Select.Group>
								<Select.Item value="unbounded" label={text.scheduleRepeatUnbounded}>{text.scheduleRepeatUnbounded}</Select.Item>
								<Select.Item value="finite" label={text.scheduleRepeatFinite}>{text.scheduleRepeatFinite}</Select.Item>
							</Select.Group></Select.Content>
						</Select.Root>
					</div>
					<div class="grid gap-2 sm:grid-cols-2">
						<div class="grid gap-2">
							<Label for="schedule-expires-at">{text.scheduleExpiresAt}</Label>
							<Input id="schedule-expires-at" type="datetime-local" bind:value={scheduleDraft.expiresAt} />
						</div>
						<div class="grid gap-2">
							<Label for="schedule-max-run-count">{text.scheduleMaxRunCountLabel}</Label>
							<Input id="schedule-max-run-count" type="number" min="1" step="1" bind:value={scheduleDraft.maxRunCount} />
						</div>
					</div>
				{/if}
			</div>

			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={() => (isOpen = false)}>{text.cancel}</Button>
				<Button type="submit" disabled={!canSaveSchedule} class="gap-2">
					{#if isSavingSchedule}
						<LoaderIcon class="size-4 animate-spin" />
					{/if}
					{text.save}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
