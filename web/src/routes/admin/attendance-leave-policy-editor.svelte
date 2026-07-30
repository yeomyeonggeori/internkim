<script lang="ts">
	import { tick } from 'svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Switch } from '$lib/components/ui/switch';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import {
		daysFromMilliDays,
		isLeaveBalanceMode,
		isLeaveExpiryMode,
		isLeaveGrantCadence,
		leaveAllowedUnits,
		leaveTypeOwnsBalance,
		leaveTypeWithBalanceMode,
		milliDaysFromDays,
		optionalMilliDaysFromDays
	} from './attendance-leave-policy-model';
	import type {
		AdminPageText,
		LeaveAllowedUnit,
		LeaveBalanceMode,
		LeaveExpiryMode,
		LeaveGrantCadence,
		LeaveType
	} from './admin-types';

	type Props = {
		draft: LeaveType;
		text: AdminPageText;
		isSaving: boolean;
		validationAttempted: boolean;
		hasChanges: boolean;
		onChange: (draft: LeaveType) => void;
		onCancel: () => void;
		onRemove: () => void;
		onSave: () => void;
	};

	let {
		draft,
		text,
		isSaving,
		validationAttempted,
		hasChanges,
		onChange,
		onCancel,
		onRemove,
		onSave
	}: Props = $props();
	let scrollViewport = $state<HTMLElement | null>(null);
	let hasScrollOverflow = $state(false);
	let isScrollAtEnd = $state(true);

	const scrollFadeThreshold = 16;

	$effect(() => {
		draft.balanceMode;
		draft.grantCadence;
		draft.expiryMode;
		draft.carryoverEnabled;
		draft.allowedUnits.length;
		tick().then(updateScrollFade);
	});

	function updateScrollFade(): void {
		if (!scrollViewport) {
			hasScrollOverflow = false;
			isScrollAtEnd = true;
			return;
		}
		const overflowAmount = scrollViewport.scrollHeight - scrollViewport.clientHeight;
		const scrollRemaining = overflowAmount - scrollViewport.scrollTop;
		hasScrollOverflow = overflowAmount > scrollFadeThreshold;
		isScrollAtEnd = scrollRemaining <= 2;
	}

	function shouldShowScrollFade(): boolean {
		return hasScrollOverflow && !isScrollAtEnd;
	}

	function update(change: (current: LeaveType) => LeaveType): void {
		onChange(change(draft));
	}

	function setBalanceMode(value: string | undefined): void {
		if (isLeaveBalanceMode(value)) {
			update((current) => leaveTypeWithBalanceMode(current, value));
		}
	}

	function setGrantCadence(value: string | undefined): void {
		if (!isLeaveGrantCadence(value)) return;
		update((current) => ({
			...current,
			grantCadence: value,
			grantAmountMilliDays: value === 'none' ? 0 : current.grantAmountMilliDays
		}));
	}

	function setExpiryMode(value: string | undefined): void {
		if (!isLeaveExpiryMode(value)) return;
		update((current) => ({
			...current,
			expiryMode: value,
			expiryMonths: value === 'monthsAfterGrant' ? current.expiryMonths ?? 12 : undefined
		}));
	}

	function toggleAllowedUnit(unit: LeaveAllowedUnit): void {
		update((current) => ({
			...current,
			allowedUnits: current.allowedUnits.includes(unit)
				? current.allowedUnits.filter((value) => value !== unit)
				: [...current.allowedUnits, unit]
		}));
	}

	function balanceModeLabel(value: LeaveBalanceMode): string {
		if (value === 'annual') return text.attendanceSettings.balanceModeAnnual;
		if (value === 'separate') return text.attendanceSettings.balanceModeSeparate;
		return text.attendanceSettings.balanceModeNone;
	}

	function grantCadenceLabel(value: LeaveGrantCadence): string {
		return text.attendanceSettings.grantCadenceLabels[value];
	}

	function expiryModeLabel(value: LeaveExpiryMode): string {
		return text.attendanceSettings.expiryModeLabels[value];
	}

	function allowedUnitLabel(unit: LeaveAllowedUnit): string {
		if (unit === 'fullDay') return text.attendanceSettings.fullDay;
		if (unit === 'halfDay') return text.attendanceSettings.halfDay;
		return text.attendanceSettings.quarterDay;
	}

	function grantAmountLabel(cadence: LeaveGrantCadence): string {
		return cadence === 'monthly'
			? text.attendanceSettings.monthlyAmount
			: text.attendanceSettings.annualAmount;
	}

	function draftName(): string {
		return localizedLeaveTypeName(draft.id, draft.name, currentLocale.value);
	}
</script>

<svelte:window onresize={updateScrollFade} />

<Card.Root class="min-h-0 max-sm:mb-20 lg:h-full">
	<Card.Header>
		<div class="flex items-center gap-2">
			<Card.Title>{draftName().trim() || text.attendanceSettings.newLeave}</Card.Title>
			{#if draft.isSystem}
				<Badge variant="secondary">{text.attendanceSettings.systemBadge}</Badge>
			{/if}
		</div>
	</Card.Header>
	<Card.Content class="min-h-0 lg:flex-1">
		<div class="relative h-full min-h-0">
			<div
				bind:this={scrollViewport}
				class="h-full min-h-0 overflow-y-auto overscroll-contain pb-8 pr-1"
				data-testid="leave-policy-editor-scroll"
				onscroll={updateScrollFade}
			>
				<Field.Group>
			<Field.Field data-invalid={validationAttempted && !draft.name.trim()}>
				<Field.Label for="leave-name">{text.attendanceSettings.name}</Field.Label>
				<Input
					id="leave-name"
					value={draftName()}
					oninput={(event) =>
						update((current) => ({ ...current, name: event.currentTarget.value }))}
					disabled={isSaving}
					placeholder={text.attendanceSettings.namePlaceholder}
					aria-invalid={validationAttempted && !draft.name.trim()}
				/>
			</Field.Field>

			<Field.Field orientation="horizontal">
				<Field.Content>
					<Field.Label for="leave-paid">{text.attendanceSettings.paid}</Field.Label>
					<Field.Description>{text.attendanceSettings.paidDescription}</Field.Description>
				</Field.Content>
				<Switch
					id="leave-paid"
					checked={draft.paid}
					onCheckedChange={(checked) => update((current) => ({ ...current, paid: checked }))}
					disabled={isSaving}
				/>
			</Field.Field>

			<Field.Field>
				<Field.Label for="leave-balance-mode">{text.attendanceSettings.balanceMode}</Field.Label>
				<Select.Root
					type="single"
					value={draft.balanceMode}
					onValueChange={setBalanceMode}
					disabled={isSaving}
				>
					<Select.Trigger id="leave-balance-mode">
						{balanceModeLabel(draft.balanceMode)}
					</Select.Trigger>
					<Select.Content>
						<Select.Group>
							<Select.Item value="annual">{text.attendanceSettings.balanceModeAnnual}</Select.Item>
							<Select.Item value="separate">{text.attendanceSettings.balanceModeSeparate}</Select.Item>
							<Select.Item value="none">{text.attendanceSettings.balanceModeNone}</Select.Item>
						</Select.Group>
					</Select.Content>
				</Select.Root>
			</Field.Field>

			{#if draft.balanceMode === 'annual' && draft.id !== 'annual'}
				<p class="text-sm text-muted-foreground">
					{text.attendanceSettings.annualBalanceDescription}
				</p>
			{:else if leaveTypeOwnsBalance(draft)}
				<Field.Field orientation="horizontal">
					<Field.Content>
						<Field.Label for="leave-include-in-summary">
							{text.attendanceSettings.includeInSummary}
						</Field.Label>
						<Field.Description>
							{text.attendanceSettings.includeInSummaryDescription}
						</Field.Description>
					</Field.Content>
					<Switch
						id="leave-include-in-summary"
						checked={draft.includeInSummary}
						onCheckedChange={(checked) =>
							update((current) => ({ ...current, includeInSummary: checked }))}
						disabled={isSaving}
					/>
				</Field.Field>

				<div class="grid gap-4 md:grid-cols-2">
					<Field.Field>
						<Field.Label for="leave-grant-cadence">
							{text.attendanceSettings.grantCadence}
						</Field.Label>
						<Select.Root
							type="single"
							value={draft.grantCadence}
							onValueChange={setGrantCadence}
							disabled={isSaving}
						>
							<Select.Trigger id="leave-grant-cadence">
								{grantCadenceLabel(draft.grantCadence)}
							</Select.Trigger>
							<Select.Content>
								<Select.Group>
									<Select.Item value="annual">{text.attendanceSettings.grantCadenceLabels.annual}</Select.Item>
									<Select.Item value="monthly">{text.attendanceSettings.grantCadenceLabels.monthly}</Select.Item>
									<Select.Item value="none">{text.attendanceSettings.grantCadenceLabels.none}</Select.Item>
								</Select.Group>
							</Select.Content>
						</Select.Root>
					</Field.Field>
					{#if draft.grantCadence !== 'none'}
					<Field.Field>
						<Field.Label for="leave-amount">
							{grantAmountLabel(draft.grantCadence)}
						</Field.Label>
						<Input
							id="leave-amount"
							type="number"
							min="0"
							step="0.25"
							value={daysFromMilliDays(draft.grantAmountMilliDays)}
							oninput={(event) =>
								update((current) => ({
									...current,
									grantAmountMilliDays: milliDaysFromDays(event.currentTarget.value)
								}))}
							disabled={isSaving}
						/>
					</Field.Field>
					{/if}
				</div>

				<div class="grid gap-4 md:grid-cols-2">
				<Field.Field>
					<Field.Label for="leave-expiry-mode">{text.attendanceSettings.expiryMode}</Field.Label>
					<Select.Root
						type="single"
						value={draft.expiryMode}
						onValueChange={setExpiryMode}
						disabled={isSaving}
					>
						<Select.Trigger id="leave-expiry-mode">
							{expiryModeLabel(draft.expiryMode)}
						</Select.Trigger>
						<Select.Content>
							<Select.Group>
								<Select.Item value="fiscalYearEnd">{text.attendanceSettings.expiryModeLabels.fiscalYearEnd}</Select.Item>
								<Select.Item value="monthsAfterGrant">{text.attendanceSettings.expiryModeLabels.monthsAfterGrant}</Select.Item>
								<Select.Item value="none">{text.attendanceSettings.expiryModeLabels.none}</Select.Item>
							</Select.Group>
						</Select.Content>
					</Select.Root>
				</Field.Field>
				{#if draft.expiryMode === 'monthsAfterGrant'}
					<Field.Field
						data-invalid={validationAttempted &&
							(!draft.expiryMonths || draft.expiryMonths < 1)}
					>
						<Field.Label for="leave-expiry-months">
							{text.attendanceSettings.expiryMonths}
						</Field.Label>
						<Input
							id="leave-expiry-months"
							type="number"
							min="1"
							step="1"
							value={draft.expiryMonths ?? 12}
							oninput={(event) =>
								update((current) => ({
									...current,
									expiryMonths: Math.max(0, Math.round(Number(event.currentTarget.value)))
								}))}
							disabled={isSaving}
							aria-invalid={validationAttempted &&
								(!draft.expiryMonths || draft.expiryMonths < 1)}
						/>
					</Field.Field>
				{/if}
				</div>

				<Field.Field orientation="horizontal">
				<Field.Content>
					<Field.Label for="leave-carryover">{text.attendanceSettings.carryover}</Field.Label>
					<Field.Description>{text.attendanceSettings.carryoverDescription}</Field.Description>
				</Field.Content>
				<Switch
					id="leave-carryover"
					checked={draft.carryoverEnabled}
					onCheckedChange={(checked) =>
						update((current) => ({
							...current,
							carryoverEnabled: checked,
							carryoverLimitMilliDays: checked
								? current.carryoverLimitMilliDays
								: undefined
						}))}
					disabled={isSaving}
				/>
				</Field.Field>

				{#if draft.carryoverEnabled}
				<Field.Field>
					<Field.Label for="leave-carryover-limit">
						{text.attendanceSettings.carryoverLimit}
					</Field.Label>
					<Field.Description>
						{text.attendanceSettings.carryoverLimitDescription}
					</Field.Description>
					<Input
						id="leave-carryover-limit"
						type="number"
						min="0"
						step="0.25"
						value={draft.carryoverLimitMilliDays === undefined
							? ''
							: daysFromMilliDays(draft.carryoverLimitMilliDays)}
						oninput={(event) =>
							update((current) => ({
								...current,
								carryoverLimitMilliDays: optionalMilliDaysFromDays(
									event.currentTarget.value
								)
							}))}
						disabled={isSaving}
					/>
				</Field.Field>
				{/if}
			{/if}

			<Field.Set>
				<Field.Group>
					{#each leaveAllowedUnits as unit (unit)}
						<Field.Field orientation="horizontal">
							<Field.Content>
								<Field.Label for={`leave-unit-${unit}`}>
									{allowedUnitLabel(unit)}
								</Field.Label>
							</Field.Content>
							<Switch
								id={`leave-unit-${unit}`}
								checked={draft.allowedUnits.includes(unit)}
								onCheckedChange={() => toggleAllowedUnit(unit)}
								disabled={isSaving}
								aria-invalid={validationAttempted && draft.allowedUnits.length === 0}
							/>
						</Field.Field>
					{/each}
				</Field.Group>
			</Field.Set>

				</Field.Group>
			</div>
			{#if shouldShowScrollFade()}
				<div
					class="pointer-events-none absolute inset-x-0 bottom-0 h-8 bg-gradient-to-t from-card via-card/75 to-transparent"
					data-testid="leave-policy-editor-scroll-fade"
				></div>
			{/if}
		</div>
	</Card.Content>
	<Card.Footer class="shrink-0 justify-between" data-testid="leave-policy-editor-footer">
		<div class="flex gap-2">
			<Button variant="outline" onclick={onCancel} disabled={isSaving || !hasChanges}>
				{text.attendanceSettings.cancelChanges}
			</Button>
			{#if draft.id}
				<Button variant="destructive" onclick={onRemove} disabled={isSaving}>
					{text.attendanceSettings.remove}
				</Button>
			{/if}
		</div>
		<Button onclick={onSave} disabled={isSaving || !hasChanges}>
			{text.attendanceSettings.save}
		</Button>
	</Card.Footer>
</Card.Root>
