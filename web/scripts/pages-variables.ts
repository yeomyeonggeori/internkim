export type Declaration = { process: string; description: string; isRequiredOnPages?: boolean };

export type HeldVariables = Record<string, { type?: string }>;

const secretType = 'secret_text';

export function variablesRequiredOnPages(declarations: Record<string, Declaration>): string[] {
	return Object.entries(declarations)
		.filter(([, declaration]) => declaration.isRequiredOnPages === true)
		.map(([name]) => name)
		.sort();
}

export function refusalOfPagesVariables(required: string[], held: HeldVariables): string | null {
	const missing = required.filter((name) => !held[name]);
	const plain = required.filter((name) => held[name] && held[name].type !== secretType);
	if (missing.length === 0 && plain.length === 0) return null;
	const lines = ['the production app would answer without these settings:'];
	for (const name of missing) lines.push(`  ${name}: not set on the project`);
	for (const name of plain) lines.push(`  ${name}: set as plain text, which the next deploy discards; set it as a secret`);
	lines.push('  bunx wrangler pages secret put <NAME> --project-name <project>, then deploy again');
	return lines.join('\n');
}
