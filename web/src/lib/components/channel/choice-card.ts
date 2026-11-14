import { agentReplyTimeoutMs } from './agent-working';
import type { ChannelChoiceOption, ChannelInteraction } from './channel-api';

export type ChoiceOptionLook = 'pressable' | 'readOnly';

type DrawnOption = ChannelChoiceOption & { position: number; isDeclining: boolean };

export type ChoiceCardOption = DrawnOption & { look: ChoiceOptionLook };

export type ChoiceCard = {
	layout: 'list' | 'pair';
	options: ChoiceCardOption[];
};

export type ApprovalOptionLabels = { approve: string; reject: string };

export type PressedChoice = { key: string; pressedAt: number };

export function choiceCardOf(
	interaction: ChannelInteraction,
	readerID: string,
	pressed: PressedChoice | undefined,
	approvalLabels: ApprovalOptionLabels
): ChoiceCard | undefined {
	if (interaction.resolution || pressed) return undefined;
	const options = drawnOptions(interaction, approvalLabels);
	const layout = interaction.kind === 'approval' && options.length === 2 ? 'pair' : 'list';
	const look = isAskingReader(interaction, readerID) ? 'pressable' : 'readOnly';
	return { layout, options: options.map((option) => ({ ...option, look })) };
}

function drawnOptions(interaction: ChannelInteraction, approvalLabels: ApprovalOptionLabels): DrawnOption[] {
	return labelledOptions(interaction, approvalLabels).map((option, index) => ({
		...option,
		position: index + 1,
		isDeclining: option.key === 'reject'
	}));
}

function labelledOptions(interaction: ChannelInteraction, approvalLabels: ApprovalOptionLabels): ChannelChoiceOption[] {
	return interaction.options
		.map((option) => ({ ...option, label: option.label || approvalLabelOf(option.key, approvalLabels) }))
		.filter((option) => option.label !== '');
}

function approvalLabelOf(key: string, approvalLabels: ApprovalOptionLabels): string {
	if (key === 'approve') return approvalLabels.approve;
	if (key === 'reject') return approvalLabels.reject;
	return '';
}

function isAskingReader(interaction: ChannelInteraction, readerID: string): boolean {
	return interaction.targetID !== '' && interaction.targetID === readerID;
}

export function pressedChoicesStillAwaited(
	pressed: ReadonlyMap<string, PressedChoice>,
	now: number
): ReadonlyMap<string, PressedChoice> {
	const awaited = [...pressed].filter(([, choice]) => now - choice.pressedAt < agentReplyTimeoutMs);
	return awaited.length === pressed.size ? pressed : new Map(awaited);
}

export function textWithoutOptionLines(text: string, options: readonly Pick<ChannelChoiceOption, 'label'>[]): string {
	const labelled = options.filter((option) => option.label !== '');
	if (labelled.length === 0) return text;
	const lines = text.split('\n');
	const firstOptionLine = lines.length - labelled.length;
	if (firstOptionLine < 0) return text;
	const isEveryOptionLine = labelled.every(
		(option, index) => lines[firstOptionLine + index] === `${index + 1}. ${option.label}`
	);
	if (!isEveryOptionLine) return text;
	const linesBefore = lines.slice(0, firstOptionLine);
	if (linesBefore.at(-1) === '') linesBefore.pop();
	return linesBefore.join('\n');
}
