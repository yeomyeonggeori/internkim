# Navigation measurements / 화면 전환 측정

Run both revisions as production Cloudflare Workers against the same disposable local Supabase seed. Do not use Vite development servers, production accounts, or reset somebody else's local stack. Build and serve each independently, then run one browser workload at a time:

```sh
bun run build
bunx wrangler pages dev .svelte-kit/cloudflare --ip 127.0.0.1 --port 5301 --inspector-port 9331
SUPABASE_URL=http://127.0.0.1:55321 bun scripts/measure-navigation.ts \
  http://127.0.0.1:5300 http://127.0.0.1:5301 \
  tests/performance/navigation-desktop.json 12 desktop
```

Supply the local publishable/secret keys through the environment and the same local bindings to each Worker. The runner refuses non-loopback endpoints, signs in as the seeded `member1@example.com`, adds 120 tasks and two calendar events, and removes those records in its finalizer. Only use a disposable test database. It alternates version order, discards one warmup pair, and uses a fresh authenticated browser context for each repetition. Scenarios within that context measure a warm revisit, shallow week selection, refresh, calendar locale/search, history and attendance administration switches. The browser cache is fresh at the beginning of each context; previously visited pages retain normal browser caches.

Use `mobile` for a 390×844 touch viewport with Chromium CPU slowdown 4. Desktop is 1440×1000 at CPU rate 1. Neither profile emulates physical device performance or measures the native Capacitor runtime. Loopback requests are unthrottled; the remote holidays provider is replaced by an equal empty response on both versions to remove outside variability. Keep database data, auth, timezone and viewport equal. Do not run builds, other browser tests, or another benchmark concurrently.

Timings start at the Playwright action, including actionability and observation overhead. `feedbackMilliseconds` is the first observed visible shell/content; already displayed content can remain visible immediately. `readyMilliseconds` is observed scenario readiness: task `data-task-ready=true` plus the owned current-week fixture card and two animation frames; previous-week URL plus its rendered selected-date label and ready flag; rendered seeded calendar event plus two frames or locale title, team summary, enabled administration refresh button. A warm snapshot being visible does not prove its background refresh has completed. Before each refresh, the runner changes the owned task/event title to a unique equal-length round marker outside the timed window. Refresh readiness requires that new title to appear on its existing card/event plus two animation frames; it therefore proves fresh UI rather than response headers or old visible content. Both revisions use the same existing DOM hooks, without baseline application instrumentation. The task is returned to its current week before refresh, outside the timed window. Intent prefetch uses 500 ms of hover preparation outside the timed click window; report that preparation alongside its click readiness, and compare cold first selection separately. Report median and observed min/max with sample count, not an asserted population p95 from twelve samples.

Request counts include all page requests through readiness plus 300 ms. `toolRequests` counts `/tools/` calls; `encodedBytes` is CDP `Network.loadingFinished.encodedDataLength` received in that window. It includes protocol overhead, excludes unfinished and websocket traffic, and is not the same as compression-normalized bundle size. Raw request paths omit tokens and query parameters. Results include exact browser version, baseline commit, host CPU model/count and per-sample host load averages. Background work outside the coordinated task lanes remains an uncontrolled source of variation on the shared Mac. Record failures separately; never turn a timeout into a faster successful value.

두 버전을 같은 격리 로컬 DB에 연결한 프로덕션 Worker로 실행합니다. 원본 DB나 운영 데이터는 초기화하지 마세요. 버전 순서를 교대하고 준비 실행 한 쌍을 제외한 뒤 각 시나리오의 중앙값·최소·최대와 표본 수를 비교합니다. 캐시가 빈 인증된 브라우저 문서 진입과 같은 문서 안의 재방문을 구분합니다. 120개 업무와 두 일정 fixture는 끝나면 지워집니다. 배경 갱신 중 보여지는 스냅샷과 서버 최신 데이터 완료를 구분하며, 로컬 DB와 동일한 휴일 응답을 사용한 수치를 운영 환경 전체의 개선율로 일반화하지 않습니다. `mobile`은 터치 화면·CPU4 시뮬레이션이며 실제 네이티브 앱 측정이 아닙니다. 빌드나 다른 브라우저 테스트와 동시에 실행하지 마세요.

Protocol version 2 requires real fixture content for initial task readiness and a new title marker for refresh. Files named `quarantined-*-v1.json` used empty-board/response-header probes and are retained only for audit; exclude every value from final comparisons.

Protocol version 3 includes canonical navigation links and bounded idle administration warming; `attendance.management_direct_first` separately measures a direct cold jump. Refresh title mutations emit the normal realtime broadcast on both builds. Thus `task.refresh` measures refresh after a remote change with realtime enabled; its requests can include the scheduled realtime refresh, and must not be described as a pure manual-refresh read count.

`measure-task-restart.ts` uses 120 owned current-week fixtures and a separate new persistent Chromium profile for each version/repetition. It completely closes that browser process before changing the fixture title, then launches a new process with the same profile. It records structure, first real card and newest marker plus two frames separately; the first cached card can be older than the newest marker. A fresh-storage phase uses a genuinely new profile. API durations include HTTP/auth/application/body work and are not database SQL timings. Use the same desktop/mobile arguments as the navigation runner. Version 2 raw results are retained with `-v2` filenames and are not pooled with version 3.

`task.revisit` and `navigation.forward` measure a rendered scoped snapshot plus the fixture card; they do not measure completion of the background server refresh.

측정 버전 2는 업무 최초 진입에서 실제 fixture 카드, 갱신에서 타이머 밖에서 변경한 새 제목과 두 렌더 프레임을 확인합니다. `quarantined-*-v1.json`은 잘못된 준비 완료 기준으로 수집한 감사용 기록이며 최종 수치에 사용하지 않습니다. 관리 화면의 의도 기반 미리 읽기는 클릭 전에 500ms를 사용하므로 첫 진입과 준비 후 클릭을 따로 비교합니다.
