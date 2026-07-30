<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import type {
		AdminPageText,
		LeaveBalanceTrackingMode
	} from './admin-types';

	type BalanceTrackingProps = {
		mode: LeaveBalanceTrackingMode;
		savedMode: LeaveBalanceTrackingMode;
		isSaving: boolean;
		text: AdminPageText;
		onChange: (mode: LeaveBalanceTrackingMode) => void;
		onCancel: () => void;
		onSave: () => void;
	};

	let {
		mode,
		savedMode,
		isSaving,
		text,
		onChange,
		onCancel,
		onSave
	}: BalanceTrackingProps = $props();

	const hasChanges = $derived(mode !== savedMode);
	const copy = $derived(
		currentLocale.value === 'ko'
			? {
					title: '휴가 운영 방식',
					description: '회사 전체의 휴가 잔여량 관리 방식을 선택합니다.',
					managed: '휴가 일수 관리',
					managedDescription: '직원별 잔여량을 계산하고 신청 시 예약·차감합니다.',
					unlimited: '휴가 자유 사용',
					unlimitedDescription: '잔여량 제한이나 차감 없이 사용·승인 대기 일수만 기록합니다.',
					save: '운영 방식 저장'
				}
			: {
					title: 'Leave operation mode',
					description: 'Choose how leave balances are managed across the company.',
					managed: 'Manage leave balances',
					managedDescription: 'Calculate employee balances and reserve and deduct leave when requested.',
					unlimited: 'Unlimited leave',
					unlimitedDescription: 'Track used and pending leave without limiting or deducting a balance.',
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
