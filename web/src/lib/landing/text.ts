export const landingText = {
	ko: {
		title: '김인턴 · AI 직원이 운영하는 작은 회사의 백오피스',
		description: '근태, 업무, 일정, 고객, 회사 문서가 한 앱에 있고, 김인턴이 그 기록을 보고 직접 처리합니다.',
		brand: '김인턴',
		navigation: { docs: '문서', github: 'GitHub', signIn: '로그인', changeLanguage: '언어 바꾸기' },
		hero: {
			headline: 'AI 직원이 운영하는 작은 회사의 백오피스',
			lead: '근태, 업무, 일정, 고객, 회사 문서가 한 앱에 있고, 김인턴이 그 기록을 보고 직접 처리합니다. 화면은 익숙한 그대로 두고, 일은 말로 맡기세요.',
			start: '회사 만들기',
			signIn: '로그인',
			openSource: '모든 소프트웨어가 오픈 소스입니다'
		},
		demo: {
			label: '김인턴과 나눈 대화 예시',
			conversation: '김인턴',
			working: '김인턴이 처리하는 중',
			scenarios: [
				{
					id: 'clockIn',
					tab: '출근',
					turns: [
						{ from: 'person', text: '오늘 재택이야, 출근 기록해 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '재택 출근 기록을 남겼습니다. 2026-11-09(월) 08:13, 재택 기준으로 저장되었습니다.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'meeting',
					tab: '회의',
					turns: [
						{ from: 'person', text: '다음 주에 박예시님이랑 나 둘 다 되는 한 시간 찾아서 회의 잡아 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '두 분 모두 다음 주에 겹치지 않는 시간을 찾아 회의를 등록했습니다.\n\n- **일시**: 11월 19일(목) 14:00~15:00 (1시간)\n- **참석자**: 이샘플, 박예시\n- **일정명**: 팀 미팅 (이샘플·박예시)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(팀 미팅)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'quote',
					tab: '견적서',
					turns: [
						{ from: 'person', text: '이거 회사 양식 견적서 PDF로 만들어 줘.', file: '견적_항목.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '이샘플님, 결과부터 말씀드립니다. 견적서 PDF(견적서_예시상사.pdf)를 첨부합니다.\n\n현재 파일 상태는 다음과 같습니다.\n\n- 문서번호: Q-2026-0142 / 견적일자 2026-11-08 / 유효기간 30일\n- 공급가액 4,690,000원, 부가세 469,000원, 총 5,159,000원', file: '견적서_예시상사.pdf', steps: ['plan', 'bash(office read 견적_항목.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					tab: '자료 링크',
					turns: [
						{ from: 'person', text: '투자사에서 회사 자료를 요청했어. 투자자 범위로 보기 전용 데이터룸 링크 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '투자자 범위로 다운로드 없이 보기만 가능하고 72시간 후 만료되는 데이터룸 링크를 만들까요?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: '응, 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '보기 전용 데이터룸 링크를 만들었습니다.\n\n- **링크**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **접속 코드**: 622332\n- **범위**: 투자자 서클\n- **권한**: 보기 전용 (다운로드 불가)\n- **유효 기간**: 72시간', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: '데이터룸' } }
					]
				}
			]
		},
		problems: {
			title: '작은 회사가 AI를 쓰면 부딪히는 세 가지',
			items: [
				{ problem: '회사 설명은 매번 처음부터', answer: '회사의 기록 안에서 일합니다', body: '업무, 일정, 고객, 문서가 한곳에 있으니 김인턴은 회사의 맥락을 이미 압니다. 누가 부탁하든 회사가 쌓아 온 것에서 시작합니다.' },
				{ problem: '구독료는 사람 수만큼', answer: '회사 하나로, 쓴 만큼', body: '일마다 맞는 모델을 골라 씁니다. 좌석 구독에 든 크레딧을 쓰거나, 회사의 모델 키를 연결합니다.' },
				{ problem: '파일은 여기저기', answer: '일하는 동안 데이터룸이 찹니다', body: '일하며 만든 계약서, 견적서, 보고서가 분류되어 쌓입니다. 투자자에게는 링크 하나로 보냅니다.' }
			]
		},
		stories: {
			attendance: { kicker: '팀의 오늘', title: '모두가 매일 들르는 곳에서 시작합니다', body: '재택이라고 말하면 출근이 찍히고, 오늘 누가 나왔는지 물으면 팀이 보는 바로 그 기록에서 답이 나옵니다.', window: '근태' },
			work: { kicker: '말로 맡기는 일', title: '화면은 익숙한 그대로, 일은 말로', body: '새 고객은 미팅까지 잡힌 채 CRM에 들어가고, 회의는 캘린더에, 할 일은 업무 보드에 올라갑니다. 김인턴이 한 일도 사람이 한 일과 같은 자리에 남습니다.', window: '업무' },
			record: { kicker: '투자자에게는 링크 하나', title: '일이 남긴 기록이 곧 회사 소개입니다', body: '출근 기록, 고객, 회의, 김인턴이 만든 문서가 데이터룸을 채웁니다. 투자자가 자료를 달라고 하면 링크 하나로 회사가 실제로 돌아가는 모습을 보여 줍니다.', window: '데이터룸' }
		},
		trust: {
			title: '회사의 일을 맡길 수 있게',
			items: [
				{ title: '부탁한 사람의 권한으로만', body: '부탁한 사람이 볼 수 있는 것만 보고, 할 수 있는 것만 합니다. 남긴 기록에는 그 사람의 이름이 붙습니다.' },
				{ title: '되돌리기 어려운 일은 먼저 묻기', body: '데이터룸 링크를 만드는 일처럼 되돌리기 어려운 일은 하기 전에 부탁한 사람에게 확인을 받습니다.' },
				{ title: '우리가 사라져도', body: '모든 소프트웨어가 공개되어 있고, AI 동료는 회사가 가진 컴퓨터에서 돌아갑니다.' }
			]
		},
		plans: {
			title: '시작과 요금',
			lead: '익숙한 화면부터 쓰고, 매주 더 많은 일을 맡기세요.',
			pay: {
				title: '쓰는 방식대로 냅니다',
				items: [
					{ title: '좌석 구독', body: '구성원 수만큼 구독하고, 구독에 든 크레딧으로 AI를 씁니다.' },
					{ title: '크레딧 추가', body: '많이 쓴 달에는 크레딧을 더 사서 씁니다.' },
					{ title: '회사의 모델 키', body: 'OpenRouter 키를 연결하면 모델 비용을 직접 냅니다.' }
				]
			},
			run: {
				title: 'AI 동료가 일할 컴퓨터',
				items: [
					{ title: '김미니', body: '김인턴이 설치된 채로 도착하는 작은 컴퓨터입니다. 전원과 인터넷만 연결하면 됩니다.' },
					{ title: '회사 컴퓨터', body: '늘 켜 둘 리눅스 서버나 Mac에 명령 두 줄로 설치합니다.' },
					{ title: '직접 운영', body: '앱과 데이터베이스까지 회사가 직접 운영할 수 있습니다.' }
				],
				guide: '셀프호스팅 안내'
			}
		},
		closing: { title: '오늘 회사를 만들고 동료를 초대하세요', quickstart: '빠른 시작 읽기' },
		footer: { madeBy: '여명거리가 만듭니다', license: 'Apache-2.0' }
	},
	en: {
		title: 'Intern Kim · The back office for small companies, run by an AI employee',
		description: 'Attendance, tasks, the calendar, customers and company documents live in one app, and Intern Kim reads those records and does the work.',
		brand: 'Intern Kim',
		navigation: { docs: 'Docs', github: 'GitHub', signIn: 'Sign in', changeLanguage: 'Change language' },
		hero: {
			headline: 'The back office for small companies, run by an AI employee',
			lead: 'Attendance, tasks, the calendar, customers and company documents live in one app, and Intern Kim reads those records and does the work. Keep the screens your team knows, and hand over the work by asking.',
			start: 'Create a company',
			signIn: 'Sign in',
			openSource: 'Every piece of it is open source'
		},
		demo: {
			label: 'A sample conversation with Intern Kim',
			conversation: 'Intern Kim',
			working: 'Intern Kim is working on it',
			scenarios: [
				{
					id: 'clockIn',
					tab: 'Clock in',
					turns: [
						{ from: 'person', text: 'Clock me in — I\'m working from home today.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'You\'re clocked in as of 08:13 KST at Remote.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'meeting',
					tab: 'Meeting',
					turns: [
						{ from: 'person', text: 'Find an hour next week when Park Example and I are both free, and book it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Done. I found an hour next week when you\'re both free and booked it:\n\n- **When**: Thu, Nov 19, 14:00–15:00 (1 hour)\n- **Attendees**: Lee Sample, Park Example\n- **Title**: Team meeting (Lee · Park)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(Team meeting)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'quote',
					tab: 'Quote',
					turns: [
						{ from: 'person', text: 'Turn this into a quote PDF on our company letterhead.', file: 'quote-items.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The quote is attached and filed in the data room. Details:\n\n- Number: Q-2026-0142 · dated 2026-11-08 · valid for 30 days\n- Supply 4,690,000 KRW, VAT 469,000 KRW, total 5,159,000 KRW', file: 'quote-example-co.pdf', steps: ['plan', 'bash(office read quote-items.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					tab: 'Data room link',
					turns: [
						{ from: 'person', text: 'An investor asked for our company materials. Make a view-only data room link for the investor circle.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Create a view-only data room link for the investor circle that expires after 72 hours and does not allow downloads?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: 'Yes, create it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The link is ready.\n\n- **Link**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **Access code**: 622332\n- **Access**: read-only through the Investor circle. Downloading originals is disabled.\n- **Valid for 72 hours**: recipients enter the code and accept the confidentiality notice without an account.\n\nShare the link and code with them by your preferred channel.', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: 'Dataroom' } }
					]
				}
			]
		},
		problems: {
			title: 'Three walls a small company hits with AI',
			items: [
				{ problem: 'You explain your company from scratch, every time', answer: 'It works inside your records', body: "Tasks, the calendar, customers and documents live in one place, so Intern Kim already knows the company. Whoever asks starts from what the company has built up." },
				{ problem: 'You pay for every seat', answer: 'One company, pay for what you use', body: 'It picks the right model for each job. Use the credits in your seat subscription, or connect the company’s own model key.' },
				{ problem: 'Your files are everywhere', answer: 'The data room fills as you work', body: 'Contracts, quotes and reports made along the way are filed as you go. An investor gets one link.' }
			]
		},
		stories: {
			attendance: { kicker: 'Every day', title: 'It starts where everyone shows up', body: "Say you're working from home and you're clocked in. Ask who's in today, and the answer comes from the same records the team sees.", window: 'Attendance' },
			work: { kicker: 'Just ask', title: 'Familiar screens, work done by asking', body: 'A new lead goes into the CRM with the meeting booked, a meeting lands on the calendar, a to-do lands on the board. What Intern Kim does sits where people would have put it.', window: 'Tasks' },
			record: { kicker: 'Always ready', title: 'The work becomes the company profile', body: 'Clock-ins, customers, meetings and the documents Intern Kim makes fill the data room. When an investor asks, one link shows the company as it actually runs.', window: 'Data room' }
		},
		trust: {
			title: 'Work you can hand over',
			items: [
				{ title: 'Only what the asker may do', body: 'It sees what the person who asked can see and does what they can do. What it leaves behind carries their name.' },
				{ title: 'It asks before the hard-to-undo', body: 'Before something hard to undo, such as creating a data room link, it checks with the person who asked.' },
				{ title: 'Even if we disappear', body: 'Every piece is open source, and the AI coworker runs on a computer your company owns.' }
			]
		},
		plans: {
			title: 'Getting started and pricing',
			lead: 'Start with the screens your team knows, and hand over more every week.',
			pay: {
				title: 'Pay the way you use it',
				items: [
					{ title: 'Per seat', body: 'Subscribe for each member, and the subscription includes credits for AI.' },
					{ title: 'More credits', body: 'In a busy month, buy more credits.' },
					{ title: 'Your own model key', body: 'Connect an OpenRouter key and pay the model costs yourself.' }
				]
			},
			run: {
				title: 'A computer for your AI coworker',
				items: [
					{ title: 'Kim mini', body: 'A small computer that arrives with Intern Kim installed. Plug in power and the internet.' },
					{ title: 'Your computer', body: 'Two commands on a Linux server or Mac that stays on.' },
					{ title: 'Run it all yourself', body: 'Your company can run the app and the database too.' }
				],
				guide: 'Self-hosting guide'
			}
		},
		closing: { title: 'Create your company today and invite your colleagues', quickstart: 'Read the quickstart' },
		footer: { madeBy: 'Made by Yeomyeonggeori', license: 'Apache-2.0' }
	}
} as const;
