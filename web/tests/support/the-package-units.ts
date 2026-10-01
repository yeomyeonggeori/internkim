import { existsSync, readFileSync } from 'node:fs';
import { basename, join } from 'node:path';

const servicesPath = join(import.meta.dir, '..', '..', '..', '.local', 'company-plane', 'services.json');

type AStartedService = { name: string; arguments: string[] };
type AFlag = { name: string; value: string; isJoinedToItsValue: boolean };

const flagName = /^--?[A-Za-z][A-Za-z0-9-]*$/;
const flagJoinedToItsValue = /^(--?[A-Za-z][A-Za-z0-9-]*)=(.*)$/;

function theServices(): AStartedService[] {
	if (!existsSync(servicesPath)) {
		throw new Error(`${servicesPath} is missing; tools/prepare-company-plane writes it from the package's units`);
	}
	return JSON.parse(readFileSync(servicesPath, 'utf8')) as AStartedService[];
}

function refuse(program: string, what: string): never {
	throw new Error(`the package starts ${program} ${what}, and this reader will not guess what it means`);
}

function theFlagsThatStart(program: string): AFlag[] {
	const service = theServices().find((candidate) => basename(candidate.arguments[0] ?? '') === program);
	if (!service) throw new Error(`the package's units start no ${program}`);
	const [, ...rest] = service.arguments;
	const flags: AFlag[] = [];
	let index = 0;
	while (index < rest.length) {
		const joined = flagJoinedToItsValue.exec(rest[index]);
		if (joined) {
			flags.push({ name: joined[1], value: joined[2], isJoinedToItsValue: true });
			index += 1;
			continue;
		}
		const name = rest[index];
		const value = rest[index + 1];
		if (!flagName.test(name)) refuse(program, `with a bare ${name} that follows no flag`);
		if (value === undefined || flagName.test(value)) refuse(program, `with ${name} and no value`);
		flags.push({ name, value, isJoinedToItsValue: false });
		index += 2;
	}
	return flags;
}

export function theArgumentsThatStart(
	program: string,
	shared: Record<string, string>,
	sandboxOnly: Record<string, string> = {}
): string[] {
	const startedFlags = theFlagsThatStart(program);
	const started = startedFlags.map((flag) => flag.name);
	for (const name of Object.keys(shared)) {
		if (started.includes(name)) continue;
		throw new Error(`this sandbox starts ${program} with ${name} as if the package did, and it does not`);
	}
	for (const name of Object.keys(sandboxOnly)) {
		if (!started.includes(name)) continue;
		throw new Error(
			`the package now starts ${program} with ${name}, so it is no longer this sandbox's own: ` +
				`give it the package's value rather than a sandbox-only one`
		);
	}
	const fromThePackage = startedFlags.flatMap(({ name, isJoinedToItsValue }) => {
		const value = shared[name];
		if (value === undefined) {
			throw new Error(
				`the package starts ${program} with ${name} and this sandbox has no value for it, ` +
					`so the plane it brings up is not the plane a company runs`
			);
		}
		return isJoinedToItsValue ? [`${name}=${value}`] : [name, value];
	});
	return [...fromThePackage, ...Object.entries(sandboxOnly).flat()];
}
