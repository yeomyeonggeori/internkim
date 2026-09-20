<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { displayPersonName } from '$lib/person-name.svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import { getAttendanceState } from '../attendance-context.svelte';
	import type { AttendanceText } from '../text';
	import type { AttendanceRecordAdditionState } from './attendance-record-addition.svelte';
	import { attendanceWriteIntent, attendanceWriteSubmitLabel } from './attendance-write-notice';

	type Props = {
		text: AttendanceText;
		addition: AttendanceRecordAdditionState;
	};

	let { text, addition }: Props = $props();

	const attendance = getAttendanceState();
	const members = $derived(attendance.summary?.members ?? []);
	const locations = $derived(attendance.summary?.locations ?? []);
	const canChoosePerson = $derived(attendance.summary?.isAdmin === true);
	const outcome = $derived(addition.outcome);
	const noticeMessage = $derived(attendanceWriteIntent(outcome, text.records));
	const submitLabel = $derived(
		attendanceWriteSubmitLabel(outcome, text.records, text.records.addSubmit)
	);
	const personName = $derived(
		displayPersonName(members.find((member) => member.email === addition.email)?.displayName ?? addition.email)
	);
	const locationName = $derived(
		locations.find((location) => location.id === addition.locationID)?.name ?? text.location
	);

	function updateStartTimeInput(event: Event): void {
		if (!(event.currentTarget instanceof HTMLInputElement)) return;
		event.currentTarget.value = addition.updateStartTime(event.currentTarget.value);
	}

	function updateEndTimeInput(event: Event): void {
		if (!(event.currentTarget instanceof HTMLInputElement)) return;
		event.currentTarget.value = addition.updateEndTime(event.currentTarget.value);
	}
</script>

<Dialog.Root open={addition.isOpen} onOpenChange={(open) => (open ? undefined : addition.close())}>
	<Dialog.Content class="sm:max-w-md" closeLabel={text.close}>
		<Dialog.Header>
			<Dialog.Title>{text.records.addTitle}</Dialog.Title>
			<Dialog.Description>{text.records.addDescription}</Dialog.Description>
		</Dialog.Header>

		<div class="grid gap-3 py-2" data-testid="attendance-record-add-fields">
			{#if canChoosePerson}
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					<span>{text.records.person}</span>
					<Select.Root
						type="single"
						value={addition.email}
						onValueChange={(value) => (addition.email = value)}
						disabled={addition.isSaving}
					>
						<Select.Trigger class="w-full">{personName}</Select.Trigger>
						<Select.Content>
							<Select.Group>
								{#each members as member (member.email)}
									<Select.Item value={member.email} label={displayPersonName(member.displayName)}>
										{displayPersonName(member.displayName)}
									</Select.Item>
								{/each}
							</Select.Group>
						</Select.Content>
					</Select.Root>
				</label>
			{/if}

			<label class="grid gap-1 text-xs font-medium text-muted-foreground">
				<span>{text.records.date}</span>
				<Input type="date" bind:value={addition.localDate} disabled={addition.isSaving} />
			</label>

			<div class="grid grid-cols-2 gap-2">
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					<span>{text.records.startTime}</span>
					<Input
						type="time"
						value={addition.startTime}
						disabled={addition.isSaving}
						oninput={updateStartTimeInput}
					/>
				</label>
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					<span>{text.records.endTime}</span>
					<Input
						type="time"
						value={addition.endTime}
						disabled={addition.isSaving}
						oninput={updateEndTimeInput}
					/>
				</label>
			</div>

			{#if addition.startTime !== '' && locations.length > 0}
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					<span>{text.location}</span>
					<Select.Root
						type="single"
						value={addition.locationID}
						onValueChange={(value) => (addition.locationID = value)}
						disabled={addition.isSaving}
					>
						<Select.Trigger class="w-full">{locationName}</Select.Trigger>
						<Select.Content>
							<Select.Group>
								{#each locations as location (location.id)}
									<Select.Item value={location.id} label={location.name}>{location.name}</Select.Item>
								{/each}
							</Select.Group>
						</Select.Content>
					</Select.Root>
				</label>
			{/if}

			<label class="grid gap-1 text-xs font-medium text-muted-foreground">
				<span>{text.records.reasonOptional}</span>
				<Textarea
					bind:value={addition.reason}
					placeholder={text.records.reasonPlaceholder}
					disabled={addition.isSaving}
					class="min-h-16 text-sm"
				/>
			</label>

			{#if addition.isSpanInverted}
				<p class="text-xs text-destructive" data-testid="attendance-record-add-span-error">
					{text.records.endBeforeStart}
				</p>
			{:else if addition.hasTimeStillToCome}
				<p class="text-xs text-destructive" data-testid="attendance-record-add-future-error">
					{text.records.timeStillToCome}
				</p>
			{:else if addition.isEndTimeMissing}
				<p class="text-xs text-muted-foreground" data-testid="attendance-record-add-end-required">
					{text.records.endRequiredOnPastDay}
				</p>
			{:else}
				<p class="text-xs text-muted-foreground" data-testid="attendance-record-add-notice">
					{noticeMessage}
				</p>
			{/if}
			{#if addition.errorMessage}
				<p class="text-xs text-destructive">{addition.errorMessage}</p>
			{/if}
		</div>

		<Dialog.Footer>
			<Button variant="outline" disabled={addition.isSaving} onclick={() => addition.close()}>
				{text.cancel}
			</Button>
			<Button
				disabled={!addition.canSubmit}
				onclick={() => addition.submit()}
				data-testid="attendance-record-add-submit"
			>
				{submitLabel}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
