<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Switch } from '$lib/components/ui/switch';
	import type {
		AdminPageText,
		AttendanceWorkBreakPeriod,
		AttendanceWorkPolicyRevision
	} from './admin-types';
	import {
		copyAttendanceWorkPolicyRevision,
		fixedAttendanceTargetMinutes,
		setAttendanceWeeklyTargetMinutes,
		setAttendanceWorkingWeekdays
	} from './attendance-work-policy-model';

	type Props = {
		revision: AttendanceWorkPolicyRevision;
		text: AdminPageText;
		disabled: boolean;
		onChange: (revision: AttendanceWorkPolicyRevision) => void;
	};

	let { revision, text, disabled, onChange }: Props = $props();

	const weekdayLabels = $derived(text.workSettings.weekdays);

	function update(mutator: (next: AttendanceWorkPolicyRevision) => void): void {
		const next = copyAttendanceWorkPolicyRevision(revision);
		mutator(next);
		synchronizeFixedTarget(next);
		onChange(next);
	}

	function synchronizeFixedTarget(next: AttendanceWorkPolicyRevision): void {
		if (next.workMode !== 'fixed') return;
		next.dailyTargetMinutes = fixedAttendanceTargetMinutes(
			next.fixedStartTime,
			next.fixedEndTime,
			next.breakPeriods
		);
		next.weeklyTargetMinutes = next.dailyTargetMinutes * next.workingWeekdays.length;
		next.referenceStartTime = next.fixedStartTime;
	}

	function toggleWeekday(weekday: number): void {
		const weekdays = revision.workingWeekdays.includes(weekday)
			? revision.workingWeekdays.filter((value) => value !== weekday)
			: [...revision.workingWeekdays, weekday];
		onChange(setAttendanceWorkingWeekdays(revision, weekdays));
	}

	function updateBreakPeriod(
		index: number,
		key: keyof AttendanceWorkBreakPeriod,
		value: string
	): void {
		update((next) => {
			next.breakPeriods[index] = { ...next.breakPeriods[index], [key]: value };
		});
	}

	function addBreakPeriod(): void {
		update((next) => {
			next.breakPeriods = [...next.breakPeriods, { startTime: '15:00', endTime: '15:15' }];
		});
	}

	function removeBreakPeriod(index: number): void {
		update((next) => {
			next.breakPeriods = next.breakPeriods.filter((_, currentIndex) => currentIndex !== index);
		});
	}

</script>

<div class="space-y-5">
	<div class="grid gap-5 md:grid-cols-2">
		{#if revision.workMode !== 'autonomous'}
			<Field.Field class="md:col-span-2">
				<Field.Label>{text.workSettings.workingWeekdays}</Field.Label>
				<div class="flex flex-wrap gap-2">
					{#each weekdayLabels as label, index (label)}
						<Button
							type="button"
							variant={revision.workingWeekdays.includes(index + 1) ? 'default' : 'outline'}
							size="sm"
							class="size-9 p-0"
							aria-pressed={revision.workingWeekdays.includes(index + 1)}
							{disabled}
							onclick={() => toggleWeekday(index + 1)}
						>
							{label}
						</Button>
					{/each}
				</div>
			</Field.Field>
		{/if}

		{#if revision.workMode === 'flexible'}
			<Field.Field>
				<Field.Label for="work-weekly-target">{text.workSettings.weeklyTarget}</Field.Label>
				<div class="relative">
					<Input
						id="work-weekly-target"
						type="number"
						min="0.25"
						max="167"
						step="0.25"
						value={revision.weeklyTargetMinutes / 60}
						oninput={(event) =>
							onChange(
								setAttendanceWeeklyTargetMinutes(
									revision,
									Math.round(Number(event.currentTarget.value) * 60)
								)
							)}
						{disabled}
					/>
					<span class="pointer-events-none absolute right-3 top-2.5 text-xs text-muted-foreground">
						{text.workSettings.hourUnit}
					</span>
				</div>
			</Field.Field>
			<Field.Field>
				<Field.Label for="work-reference-start">{text.workSettings.referenceStart}</Field.Label>
				<Input
					id="work-reference-start"
					type="time"
					value={revision.referenceStartTime}
					oninput={(event) => update((next) => (next.referenceStartTime = event.currentTarget.value))}
					{disabled}
				/>
			</Field.Field>
			<Field.Field orientation="horizontal">
				<Field.Content>
					<Field.Label for="work-core-enabled">{text.workSettings.coreTime}</Field.Label>
					<Field.Description>{text.workSettings.coreTimeEnabled}</Field.Description>
				</Field.Content>
				<Switch
					id="work-core-enabled"
					checked={revision.coreTimeEnabled}
					onCheckedChange={(checked) => update((next) => (next.coreTimeEnabled = checked))}
					{disabled}
				/>
			</Field.Field>
			{#if revision.coreTimeEnabled}
				<Field.Field>
					<Field.Label>{text.workSettings.coreTime}</Field.Label>
					<div class="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
						<Input
							type="time"
							aria-label={`${text.workSettings.coreTime} ${text.workSettings.start}`}
							value={revision.coreStartTime}
							oninput={(event) => update((next) => (next.coreStartTime = event.currentTarget.value))}
							{disabled}
						/>
						<span class="text-muted-foreground">–</span>
						<Input
							type="time"
							aria-label={`${text.workSettings.coreTime} ${text.workSettings.end}`}
							value={revision.coreEndTime}
							oninput={(event) => update((next) => (next.coreEndTime = event.currentTarget.value))}
							{disabled}
						/>
					</div>
				</Field.Field>
			{/if}
		{:else if revision.workMode === 'fixed'}
			<Field.Field class="md:col-span-2">
				<Field.Label>{text.workSettings.fixedHours}</Field.Label>
				<div class="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
					<Input
						type="time"
						aria-label={`${text.workSettings.fixedHours} ${text.workSettings.start}`}
						value={revision.fixedStartTime}
						oninput={(event) => update((next) => (next.fixedStartTime = event.currentTarget.value))}
						{disabled}
					/>
					<span class="text-muted-foreground">–</span>
					<Input
						type="time"
						aria-label={`${text.workSettings.fixedHours} ${text.workSettings.end}`}
						value={revision.fixedEndTime}
						oninput={(event) => update((next) => (next.fixedEndTime = event.currentTarget.value))}
						{disabled}
					/>
				</div>
			</Field.Field>
		{:else}
			<p class="rounded-lg bg-muted px-4 py-3 text-sm text-muted-foreground md:col-span-2">
				{text.workSettings.autonomousNotice}
			</p>
		{/if}
	</div>

	<div class="border-t pt-5">
		<div class="mb-4">
			<h3 class="text-sm font-semibold">{text.workSettings.commonRules}</h3>
			<p class="mt-1 text-xs text-muted-foreground">{text.workSettings.commonRulesDescription}</p>
		</div>
		<div class="grid gap-5 md:grid-cols-2">
			<Field.Field>
				<Field.Label>{text.workSettings.nightHours}</Field.Label>
				<div class="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
					<Input
						type="time"
						aria-label={`${text.workSettings.nightHours} ${text.workSettings.start}`}
						value={revision.nightStartTime}
						oninput={(event) => update((next) => (next.nightStartTime = event.currentTarget.value))}
						{disabled}
					/>
					<span class="text-muted-foreground">–</span>
					<Input
						type="time"
						aria-label={`${text.workSettings.nightHours} ${text.workSettings.end}`}
						value={revision.nightEndTime}
						oninput={(event) => update((next) => (next.nightEndTime = event.currentTarget.value))}
						{disabled}
					/>
				</div>
			</Field.Field>
			<div class="space-y-3">
				<Field.Label>{text.workSettings.breakPeriods}</Field.Label>
				{#each revision.breakPeriods as period, index (index)}
					<div class="grid grid-cols-[1fr_auto_1fr_auto] items-center gap-2">
						<Input
							type="time"
							aria-label={`${text.workSettings.breakPeriods} ${index + 1} ${text.workSettings.start}`}
							value={period.startTime}
							oninput={(event) => updateBreakPeriod(index, 'startTime', event.currentTarget.value)}
							{disabled}
						/>
						<span class="text-muted-foreground">–</span>
						<Input
							type="time"
							aria-label={`${text.workSettings.breakPeriods} ${index + 1} ${text.workSettings.end}`}
							value={period.endTime}
							oninput={(event) => updateBreakPeriod(index, 'endTime', event.currentTarget.value)}
							{disabled}
						/>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							aria-label={`${text.workSettings.breakPeriods} ${index + 1} ${text.workSettings.remove}`}
							{disabled}
							onclick={() => removeBreakPeriod(index)}
						>
							{text.workSettings.remove}
						</Button>
					</div>
				{/each}
				<Button type="button" variant="outline" size="sm" {disabled} onclick={addBreakPeriod}>
					{text.workSettings.addBreak}
				</Button>
			</div>
		</div>
	</div>

</div>
