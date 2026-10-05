<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import CheckIcon from '@lucide/svelte/icons/check';
	import XIcon from '@lucide/svelte/icons/x';
	import { decideApproval, type ApprovalDecision, type ApprovalOutcome, type PendingApproval } from './runs-api';
	import type { TasksText } from './text';
	import { feelHaptic } from '$lib/native-shell/haptics';

	type Props = {
		approval: PendingApproval;
		text: TasksText;
		onDecided: (outcome: ApprovalOutcome) => void;
	};

	let { approval, text, onDecided }: Props = $props();

	let decidingWith = $state<ApprovalDecision | undefined>(undefined);
	let decisionError = $state('');

	const decisionButtons: { decision: ApprovalDecision; label: string; icon: typeof CheckIcon; variant: 'default' | 'outline' | 'destructive' }[] = $derived([
		{ decision: 'approve', label: text.approveOnce, icon: CheckIcon, variant: 'default' },
		{ decision: 'reject', label: text.rejectApproval, icon: XIcon, variant: 'destructive' }
	]);

	async function decide(decision: ApprovalDecision) {
		if (decidingWith) return;
		decidingWith = decision;
		decisionError = '';
		try {
			const outcome = await decideApproval(approval.taskRun.taskRunID, decision);
			feelHaptic('success');
			onDecided(outcome);
		} catch {
			decisionError = text.approvalDecisionError;
		} finally {
			decidingWith = undefined;
		}
	}
</script>

<div class="flex flex-col gap-3">
	<div class="flex flex-wrap items-center gap-2">
		<span class="text-xs text-muted-foreground">{text.approvalQuestionLabel}</span>
		{#if approval.scope}
			<Badge variant="secondary">{approval.scope}</Badge>
		{/if}
	</div>
	<p class="text-sm whitespace-pre-wrap">{approval.question || text.approvalQuestionMissing}</p>
	<Separator />
	<div class="flex flex-wrap gap-2">
		{#each decisionButtons as decisionButton (decisionButton.decision)}
			{@const DecisionIcon = decisionButton.icon}
			<Button
				variant={decisionButton.variant}
				size="sm"
				disabled={decidingWith !== undefined}
				onclick={() => decide(decisionButton.decision)}
			>
				<DecisionIcon data-icon="inline-start" />
				{decisionButton.label}
			</Button>
		{/each}
	</div>
	{#if decisionError}
		<p class="text-sm text-destructive">{decisionError}</p>
	{/if}
</div>
