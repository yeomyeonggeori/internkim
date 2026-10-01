export type Declaration = { process: string; description: string; isRequiredOnPages?: boolean };

export type HeldVariables = Record<string, { type?: string }>;

const secretType = 'secret_text';

export function requiresPagesRuntimeVariables(projectName: string): boolean {
	return projectName !== 'internkim-docs';
}

export function variablesRequiredOnPages(declarations: Record<string, Declaration>): string[] {
	return Object.entries(declarations)
		.filter(([, declaration]) => declaration.isRequiredOnPages === true)
		.map(([name]) => name)
		.sort();
}

export type PagesEnvironmentVariables = Record<string, { type: 'secret_text'; value: string }>;

export function pagesVariablesFromVault(
	required: string[],
	valueOf: (name: string) => string
): PagesEnvironmentVariables {
	const missing = required.filter((name) => !valueOf(name));
	if (missing.length > 0) {
		throw new Error(
			`${missing.join(', ')} not set: run this through \`monkeys run @production\`, which hands it the vault`
		);
	}
	return Object.fromEntries(required.map((name) => [name, { type: secretType, value: valueOf(name) }]));
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
