import { json } from '@sveltejs/kit';

const latestReleaseBaseURL = 'https://gitlab.com/eastriver/internkim/-/releases/permalink/latest/downloads';

const platforms = [
	{
		platform: 'macos',
		label: 'macOS',
		architecture: 'Apple Silicon beta',
		status: 'available',
		url: `${latestReleaseBaseURL}/internkim-companion-beta-macos-aarch64.dmg`
	},
	{
		platform: 'windows',
		label: 'Windows',
		architecture: 'x64',
		status: 'coming_soon',
		url: ''
	},
	{
		platform: 'linux',
		label: 'Linux',
		architecture: 'x64 AppImage',
		status: 'coming_soon',
		url: ''
	}
];

export function GET() {
	return json({ platforms });
}
