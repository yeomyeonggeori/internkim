import { json } from '@sveltejs/kit';

const latestReleaseBaseURL = 'https://gitlab.com/eastriver/internkim/-/releases/permalink/latest/downloads';

const platforms = [
	{
		platform: 'macos',
		label: 'macOS',
		architecture: 'Universal',
		url: `${latestReleaseBaseURL}/internkim-companion-shell-darwin-universal.dmg`
	},
	{
		platform: 'windows',
		label: 'Windows',
		architecture: 'x64',
		url: `${latestReleaseBaseURL}/internkim-companion-shell-windows-x64.msi`
	},
	{
		platform: 'linux',
		label: 'Linux',
		architecture: 'x64 AppImage',
		url: `${latestReleaseBaseURL}/internkim-companion-shell-linux-x64.AppImage`
	}
];

export function GET() {
	return json({ platforms });
}
