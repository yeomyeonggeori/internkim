const vaultProfile = '@production';

export function setting(name: string): string {
	return process.env[name] ?? '';
}

export function requiredSetting(name: string): string {
	const value = setting(name);
	if (value) return value;
	throw new Error(`${name} is not set: run this through \`monkeys run ${vaultProfile}\`, which hands it the vault`);
}

export function rememberSetting(name: string, value: string): void {
	if (setting(name)) throw new Error(`${name} is already set; forget it before issuing another`);
	const remembered = Bun.spawnSync(['monkeys', 'remember', vaultProfile, name], {
		stdin: new TextEncoder().encode(value),
		stdout: 'inherit',
		stderr: 'inherit'
	});
	if (remembered.exitCode !== 0) throw new Error(`monkeys could not remember ${name} in ${vaultProfile}`);
}
