export const captiveText = {
	ko: {
		setupLabel: 'Kim mini 설정',
		heading: '사무실 Wi-Fi에 연결하세요',
		joinFailure: '연결하지 못했습니다. 비밀번호를 확인하고 다시 시도하세요.',
		networkLabel: '네트워크 선택',
		scanning: '주변 네트워크를 찾고 있습니다.',
		noNetworks: '주변에서 찾은 네트워크가 없습니다. 이름을 직접 입력하세요.',
		enterManually: '직접 입력',
		customNetworkLabel: '네트워크 이름',
		passwordLabel: '비밀번호',
		connect: '연결',
		networkRequired: '네트워크를 선택하거나 입력하세요.',
		alreadySubmitted: '이미 접수되었습니다.',
		unreachable: 'Kim mini에 닿지 못했습니다. Kim mini Wi-Fi에 연결되어 있는지 확인하세요.',
		joiningHeading: '연결을 시도하고 있습니다',
		joiningInstruction: '이제 사무실 Wi-Fi로 돌아가 intern.kim 을 여세요.'
	},
	en: {
		setupLabel: 'Kim mini setup',
		heading: 'Connect to your office Wi-Fi',
		joinFailure: 'Could not connect. Check the password and try again.',
		networkLabel: 'Choose a network',
		scanning: 'Looking for nearby networks.',
		noNetworks: 'No networks were found nearby. Enter the name yourself.',
		enterManually: 'Enter manually',
		customNetworkLabel: 'Network name',
		passwordLabel: 'Password',
		connect: 'Connect',
		networkRequired: 'Choose or enter a network.',
		alreadySubmitted: 'Already received.',
		unreachable: 'Could not reach Kim mini. Check that you are on the Kim mini Wi-Fi.',
		joiningHeading: 'Trying to connect',
		joiningInstruction: 'Switch back to your office Wi-Fi and open intern.kim.'
	}
} as const;

export type CaptiveLanguage = keyof typeof captiveText;

export function preferredLanguage(languages: readonly string[]): CaptiveLanguage {
	const first = languages[0]?.toLowerCase() ?? '';
	return first === 'ko' || first.startsWith('ko-') ? 'ko' : 'en';
}
