import { existsSync } from "node:fs";

type Command = {
	name: string;
	arguments: string[];
};

async function runCommand(command: Command): Promise<void> {
	const commandProcess = Bun.spawn([command.name, ...command.arguments], {
		stdout: "inherit",
		stderr: "inherit",
	});
	const exitCode = await commandProcess.exited;
	if (exitCode !== 0) {
		throw new Error(command.name + " " + command.arguments.join(" ") + " failed with exit code " + exitCode);
	}
}

if (!existsSync("../DESIGN.md")) {
	throw new Error("DESIGN.md is required at the site workspace root");
}

if (!existsSync("node_modules")) {
	await runCommand({ name: "bun", arguments: ["install"] });
}

await runCommand({ name: "bunx", arguments: ["@google/design.md", "lint", "../DESIGN.md"] });
await runCommand({ name: "bunx", arguments: ["vite", "build"] });
