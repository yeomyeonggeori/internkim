<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import * as Field from '$lib/components/ui/field';
	import * as Select from '$lib/components/ui/select';
	import {
		daysInLeaveYearStartMonth,
		leaveYearStartWithin,
		sameLeaveYearStart,
		type LeaveYearStart
	} from './leave-year-start-model';
	import type {
		AdminPageText,
		LeaveBalanceTrackingMode
	} from './admin-types';

	type BalanceTrackingProps = {
		mode: LeaveBalanceTrackingMode;
		savedMode: LeaveBalanceTrackingMode;
		yearStart: LeaveYearStart;
		savedYearStart: LeaveYearStart;
		isSaving: boolean;
		text: AdminPageText;
		onChange: (mode: LeaveBalanceTrackingMode) => void;
		onYearStartChange: (yearStart: LeaveYearStart) => void;
		onCancel: () => void;
		onSave: () => void;
	};

	let {
		mode,
		savedMode,
		yearStart,
		savedYearStart,
		isSaving,
		text,
		onChange,
		onYearStartChange,
		onCancel,
		onSave
	}: BalanceTrackingProps = $props();

	const hasChanges = $derived(
		mode !== savedMode || !sameLeaveYearStart(yearStart, savedYearStart)
	);
	const dayCount = $derived(daysInLeaveYearStartMonth(yearStart.month));
	const copy = $derived(
		currentLocale.value === 'ko'
			? {
					title: '휴가 운영 방식',
					description: '회사 전체의 휴가 잔여량 관리 방식을 선택합니다.',
					managed: '휴가 일수 관리',
					managedDescription: '직원별 잔여량을 계산하고 신청 시 예약·차감합니다.',
					unlimited: '휴가 자유 사용',
					unlimitedDescription: '잔여량 제한이나 차감 없이 사용·승인 대기 일수만 기록합니다.',
					yearStart: '휴가 연도 시작',
					yearStartDescription: '이 날부터 한 해를 셉니다. 바꾸면 기록된 휴가가 속한 해가 달라집니다.',
					month: '월',
					day: '일',
					monthNamed: (month: number) => `${month}월`,
					dayNamed: (day: number) => `${day}일`,
					save: '운영 방식 저장'
				}
			: {
					title: 'Leave operation mode',
					description: 'Choose how leave balances are managed across the company.',
					managed: 'Manage leave balances',
					managedDescription: 'Calculate employee balances and reserve and deduct leave when requested.',
					unlimited: 'Unlimited leave',
					unlimitedDescription: 'Track used and pending leave without limiting or deducting a balance.',
					yearStart: 'Leave year starts',
					yearStartDescription: 'A year is counted from this day. Changing it moves which year recorded leave falls in.',
					month: 'Month',
					day: 'Day',
					monthNamed: (month: number) =>
						new Intl.DateTimeFormat('en-US', { month: 'long' }).format(new Date(2001, month - 1, 1)),
					dayNamed: (day: number) => String(day),
					save: 'Save operation mode'
				}
	);
</script>

<Card.Root data-testid="leave-balance-tracking-settings">
	<Card.Header class="pb-3">
		<Card.Title>{copy.title}</Card.Title>
		<Card.Description>{copy.description}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-3 sm:grid-cols-2">
		<button
			type="button"
			class={[
				'rounded-lg border p-4 text-left transition-colors',
				mode === 'managed'
					? 'border-foreground bg-muted'
					: 'border-border hover:bg-muted/50'
			]}
			aria-pressed={mode === 'managed'}
			onclick={() => onChange('managed')}
			disabled={isSaving}
			data-testid="leave-balance-tracking-managed"
		>
			<span class="block font-medium">{copy.managed}</span>
			<span class="mt-1 block text-sm text-muted-foreground">
				{copy.managedDescription}
			</span>
		</button>
		<button
			type="button"
			class={[
				'rounded-lg border p-4 text-left transition-colors',
				mode === 'unlimited'
					? 'border-foreground bg-muted'
					: 'border-border hover:bg-muted/50'
			]}
			aria-pressed={mode === 'unlimited'}
			onclick={() => onChange('unlimited')}
			disabled={isSaving}
			data-testid="leave-balance-tracking-unlimited"
		>
			<span class="block font-medium">{copy.unlimited}</span>
			<span class="mt-1 block text-sm text-muted-foreground">
				{copy.unlimitedDescription}
			</span>
		</button>
		<div class="mt-5 sm:col-span-2">
			<Field.Set>
				<Field.Legend>{copy.yearStart}</Field.Legend>
				<Field.Description>{copy.yearStartDescription}</Field.Description>
				<div class="grid gap-3 sm:grid-cols-2">
					<Field.Field>
						<Field.Label for="leave-year-start-month">{copy.month}</Field.Label>
						<Select.Root
							type="single"
							value={String(yearStart.month)}
							onValueChange={(value) =>
								onYearStartChange(leaveYearStartWithin(Number(value), yearStart.day))}
							disabled={isSaving}
						>
							<Select.Trigger id="leave-year-start-month" class="w-full">
								{copy.monthNamed(yearStart.month)}
							</Select.Trigger>
							<Select.Content class="max-h-72">
								<Select.Group>
									{#each Array.from({ length: 12 }, (_, index) => index + 1) as month (month)}
										<Select.Item value={String(month)}>{copy.monthNamed(month)}</Select.Item>
									{/each}
								</Select.Group>
							</Select.Content>
						</Select.Root>
					</Field.Field>
					<Field.Field>
						<Field.Label for="leave-year-start-day">{copy.day}</Field.Label>
						<Select.Root
							type="single"
							value={String(yearStart.day)}
							onValueChange={(value) =>
								onYearStartChange(leaveYearStartWithin(yearStart.month, Number(value)))}
							disabled={isSaving}
						>
							<Select.Trigger id="leave-year-start-day" class="w-full">
								{copy.dayNamed(yearStart.day)}
							</Select.Trigger>
							<Select.Content class="max-h-72">
								<Select.Group>
									{#each Array.from({ length: dayCount }, (_, index) => index + 1) as day (day)}
										<Select.Item value={String(day)}>{copy.dayNamed(day)}</Select.Item>
									{/each}
								</Select.Group>
							</Select.Content>
						</Select.Root>
					</Field.Field>
				</div>
			</Field.Set>
		</div>
	</Card.Content>
	<Card.Footer class="justify-between border-t">
		<Button variant="outline" onclick={onCancel} disabled={isSaving || !hasChanges}>
			{text.attendanceSettings.cancelChanges}
		</Button>
		<Button onclick={onSave} disabled={isSaving || !hasChanges}>
			{copy.save}
		</Button>
	</Card.Footer>
</Card.Root>
