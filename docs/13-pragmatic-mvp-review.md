# 13 - Pragmatic MVP Review

Ngày review: 2026-10-01  
Phạm vi: tài liệu PRD, kiến trúc, delivery/testing strategy, lịch sử Git và mã hiện có.  
Mục tiêu: đánh giá liệu StayGuard có đang over-engineer cho một solo founder cần có demo production-ready sớm, đồng thời đề xuất cách tăng tốc mà vẫn giữ những lời hứa cốt lõi với khách hàng.

## 1. Kết luận điều hành

StayGuard đang **over-engineer chủ yếu ở quy trình giao hàng, số lượng quality gate và định nghĩa hoàn thành của từng story**, chứ không over-engineer ở mọi lựa chọn kỹ thuật.

Các cơ chế dưới đây là lõi của lời hứa chống thất thoát; không nên cắt để chạy nhanh:

- giá tiền bằng số nguyên VND và pricing do server tính;
- thời điểm check-in/check-out do server ghi;
- payment transfer chỉ thành paid khi server nhận sự kiện xác nhận;
- idempotency cho những lệnh thay đổi tiền/trạng thái;
- phân tách tenant và kiểm tra quyền theo building;
- không lưu hoặc không làm lộ dữ liệu định danh cá nhân.

Ngược lại, việc coi mỗi feature demo như một production subsystem hoàn chỉnh — gồm coverage gate rộng, mutation/chaos/performance suites, visual/E2E đầy đủ cho hai ngôn ngữ, ADR/plan/tracking/PR ceremony — đang làm chậm delivery nhiều hơn lợi ích trước khi có khách hàng trả tiền.

**Khuyến nghị:** không rewrite stack và không “flatten” handlers. Giữ modular monolith Go + PostgreSQL + sqlc hiện tại; giảm scope demo, làm UI theo vertical slice, và chỉ giữ automated tests cho money, tenancy, authorization và một happy path xuyên suốt.

## 2. Những gì đã được kiểm tra

### 2.1 Tài liệu

Đã đối chiếu:

- `docs/01-prd.md` — mục tiêu, features và release definition;
- `docs/02-software-architecture.md` — FR/NFR, runtime flow và architecture constraints;
- `docs/06-user-stories.md` — acceptance criteria theo từng story;
- `docs/07-delivery-plan.md` — lanes, sprint plan, feature flags và critical path;
- `docs/08-test-strategy.md` — test pyramid và quality gates;
- `docs/10-engineering-standards.md` — các non-negotiables;
- `docs/12-mvp-to-production-strategy.md` — đề xuất acceleration được Gemini ghi lại;
- `README.md` và `docs/progress.md` — trạng thái công bố.

### 2.2 Git history và implementation

Lịch sử trên `main` có các story đã merge:

- SG-001 repository/tooling/CI;
- SG-002 contract pipeline;
- SG-003 database foundation;
- SG-101 pricing engine;
- SG-102 sessions, identity, tenant context;
- SG-201 rooms/buildings read API;
- SG-203 check-in API;
- SG-205 extras và check-out API.

Các commit này đều được tạo ngày 2026-10-01. Churn API khoảng 24.5k dòng thêm và 303 dòng xóa; phần production code Go viết tay khoảng 6.1k LOC (không gồm generated code và tests). Repository hiện có 96 test files dưới `api/` và `web/`.

`make test-api` đã pass tại thời điểm review. Web hiện là shell/mocks: không có screen nghiệp vụ nào ngoài placeholder. Trong checkout hiện tại, `node_modules` không có các package đã khai báo, nên `npm test` và `npm run build` không type-check được; đây là thiếu dependency cài local, không đủ bằng chứng để kết luận source web hỏng.

## 3. Trạng thái thực tế so với tracking

Có độ lệch tài liệu cần sửa sớm:

- `README.md` còn ghi trạng thái “pre-implementation”.
- `docs/progress.md` ghi Sprint 0 “not started” và 0 points done.
- Cùng file đó lại ghi SG-001, 002, 003, 101, 102, 201, 203 và 205 là “Merged (flag off)”.

Độ lệch này dễ dẫn đến ưu tiên sai và làm AI/human reviewer hiểu nhầm nền tảng nào đã tồn tại. Trước khi triển khai tiếp, cập nhật README và progress thành source of truth khớp Git history.

Ngoài ra, `docs/12-mvp-to-production-strategy.md` đang là file untracked. Nếu nó được chọn làm định hướng chính thức, cần được review và commit ở một thay đổi riêng; không nên để policy quyết định kiến trúc nằm ngoài lịch sử Git.

## 4. Phần nào thực sự over-engineered

### 4.1 Delivery process

Kế hoạch hiện tại mô hình hoá ba lane, contract-first PR, feature flags cho từng slice, checkpoint, documentation lifecycle và nhiều test level cho 73 points/4 sprints. Cách này hợp lý khi nhiều contributor làm song song, nhưng solo founder là reviewer và integrator duy nhất thì ceremony có thể trở thành bottleneck lớn hơn coding.

Ví dụ cần giảm:

- Không bắt buộc PR dưới 400 dòng cho UI screen; giữ reviewable structure nhưng cho phép một vertical slice lớn hơn.
- Không bắt buộc ADR/bug-log/release-note cho mọi thay đổi UI nhỏ.
- Không bắt buộc visual regression, Lighthouse, axe, contract provider test, Testcontainers và E2E hai ngôn ngữ trên mọi PR.
- Không viết plan dài trước khi UI/reference code đã sẵn sàng.

### 4.2 Scope của “public demo”

PRD v0.1.0 chứa cả front-desk loop lẫn role picker, per-trial tenant lifecycle, building permissions, cash shift reconciliation, bilingual UI, accessibility, E2E và public repository hygiene. Đây là một public product preview khá đầy đủ, không phải demo sales tối thiểu 3 phút.

Vì vậy phải tách hai định nghĩa:

1. **Sales demo:** phục vụ buổi gặp khách, ưu tiên hiệu ứng chứng minh value proposition.
2. **Public PRD demo v0.1:** đáp ứng đầy đủ FR-01 tới FR-15 và các NFR được cam kết.

Không thể bỏ role picker, i18n, shift close, tenant trials và permissions mà vẫn nói là đã đáp ứng toàn bộ PRD hiện tại. Có thể hoãn chúng khỏi sales demo, nhưng cần gọi đúng tên là giảm scope/release, không phải “vẫn đúng PRD”.

### 4.3 Frontend là bottleneck thật sự

Backend core đã đi đến checkout nhưng frontend chỉ có foundation/mocks. Không thấy source asset/code Claude Design trong repository tại thời điểm review. Nếu raw Tailwind/React code thực sự có ở nơi khác, việc đưa nó vào workspace và bind vào generated API client là đòn bẩy nhanh nhất.

Không nên đầu tư trước vào generic design system hoặc ép tất cả UI vào component library. Reuse layout Tailwind có sẵn; chỉ dùng component library cho primitive khó như dialog, select, date picker nếu cần.

## 5. Phản biện các đề xuất trong `12-mvp-to-production-strategy.md`

### 5.1 Đề xuất nên chấp nhận

| Đề xuất | Đánh giá | Điều chỉnh khuyến nghị |
| --- | --- | --- |
| Giữ Go + sqlc thay vì đổi Gin/GORM | Đúng | Không rewrite stack; lợi ích tốc độ không bù chi phí chuyển đổi. |
| Reuse Claude Design Tailwind/React | Đúng, nếu asset có sẵn | Import nguyên layout rồi kết nối data/API; không xây lại pixel-by-pixel. |
| Dùng shadcn/ui chọn lọc | Đúng | Chỉ cho interactive primitive, không thay thế design language. |
| SSE thành polling | Đúng cho sales demo | Poll `GET payment` mỗi 3 giây; simulator/payment-event handler vẫn phải là server-side. |
| Giảm PR-size ceremony | Đúng | Bỏ giới hạn cứng hoặc yêu cầu waiver cho UI slice; vẫn review theo checklist ngắn. |
| Giảm CI nặng | Đúng một phần | Giữ build/lint/secrets và critical tests; chạy full integration/nightly hoặc trước release. |
| Tách web/API deployment | Có thể đúng | Chỉ chọn nếu nhanh hơn thật cho deploy; single container hiện rất đơn giản và tránh CORS/session complexity. |

### 5.2 Đề xuất không nên chấp nhận

| Đề xuất | Vì sao không nên | Phương án nhanh hơn nhưng an toàn |
| --- | --- | --- |
| Bỏ RLS | Tenant isolation là lời hứa PRD, không phải tối ưu scale. Nó cũng đã được đầu tư trong migrations. Bỏ rồi thêm lại tạo migration/risk mới. | Giữ RLS + explicit tenant filter; không mở rộng isolation suite cho mọi UI change. |
| Lưu PII plaintext | Dữ liệu CCCD/khách lưu plaintext tạo rủi ro không cần thiết, kể cả demo. | Không thu thập ID number trong demo; giữ encryption seam sẵn có cho lúc cần. |
| Bỏ idempotency, chỉ disable nút UI | Không xử lý retry mạng, reload hoặc request lặp từ client khác; double checkout/payment hại trực tiếp demo. | Giữ idempotency cho check-in, extras, checkout, payment; không nhất thiết áp dụng cho mọi read/simple setting. |
| Bỏ `Vnd` value type | Tiền là domain differentiation, wrapper đã hoàn thành và chi phí duy trì thấp. | Giữ `Vnd`, không mở rộng abstraction/value objects vô ích khác. |
| Bỏ toàn bộ tests | Demo payment sai hoặc tenant leak phá niềm tin khách hàng; manual-only khó lặp lại. | Giữ unit/golden pricing, vài integration tests cho writes, một smoke journey; manual visual QA cho UI. |
| Xóa feature flags | Feature flags có overhead nhỏ vì đã tồn tại; xóa không tạo screen nào nhanh hơn và tăng rủi ro demo partial. | Dùng một demo env bật các slice đã integration; chỉ xóa flags sau khi release ổn định. |
| Stop hand-writing OpenAPI | Contract vẫn cần một source-of-truth có review. AI generate YAML vẫn cần validate. | Dùng AI draft spec, review diff nhanh, regenerate client/mocks. |

### 5.3 Những fact cần sửa trong strategy

- Test strategy không yêu cầu 100% coverage. Nó đặt 85% Go overall và 95% cho `domain/`; do đó nhận định “100% test coverage requirement” là không chính xác.
- Kiến trúc hiện tại là modular monolith, không phải microservices/hyper-scale architecture. Vì vậy lập luận “cần bỏ để tránh microservice complexity” chưa đúng đối tượng.
- Hiện không có bằng chứng trong repo rằng Claude Design UI code đã có sẵn; đó là dependency ngoài workspace, không phải tài sản hiện hữu.
- Docker deployment hiện chỉ là multi-stage Dockerfile ngắn, không phải pipeline phức tạp. Tách deployment chỉ là lựa chọn delivery, không phải ưu tiên kiến trúc.

## 6. Kiến trúc nên giữ và cách đơn giản hoá hợp lý

### Giữ

- Một Go modular monolith và một PostgreSQL database.
- `sqlc`, migrations, OpenAPI generation và generated web client/mocks.
- Pure pricing module, `Vnd`, server clock, transactions và row locking cho check-in/payment.
- Session token hash, tenant filter/RLS, building authorization.
- Idempotency cho writes có thể tạo tiền, invoice hoặc state transition.
- Append-only/audit tối thiểu cho payment, permission và shift.

### Đơn giản hoá

- Mọi feature mới dùng cấu trúc mỏng `HTTP handler -> application service -> repository`; không thêm domain abstraction mới nếu chỉ là read CRUD.
- Không bắt buộc port/fake mới cho dependency đơn giản khi no external boundary tồn tại. Các seam đã có nên được reuse.
- Không tạo generic framework cho alert, report, billing policy hay plugin system trước khi một use case thứ hai cần nó.
- Payment status dùng polling trong demo thay SSE; API settle logic vẫn một nguồn duy nhất.
- Một tenant demo seed sẵn cho sales demo; trial clone/expiry/rate-limit để sau lớp demo nếu không công khai self-service.
- Nếu khách demo chỉ cần tiếng Việt, làm Vietnamese first. English phải quay lại trước public release để giữ cam kết PRD.

## 7. Scope khuyến nghị

### 7.1 Sales demo — vertical slice cần hoàn thành trước

Luồng phải chạy được từ đầu đến cuối:

1. Chọn hoặc dùng sẵn role receptionist trong tenant demo.
2. Xem room map.
3. Check-in một phòng theo hourly/overnight/daily; thời gian do server ghi.
4. Xem running total và thêm extras.
5. Checkout ra itemised invoice/deposit balance.
6. Chọn cash hoặc transfer; QR luôn là tài khoản owner.
7. Simulate server-confirmed transfer; trạng thái paid và phòng chuyển `TO_CLEAN`.
8. Housekeeping mark clean; phòng trở lại vacant.
9. Owner overview cho thấy revenue/payment/alert cơ bản.

Các phần này chứng minh bốn control mà khách hàng quan tâm: giá đúng, tiền về owner, thời gian không bị staff sửa, và visibility cho owner.

### 7.2 Hoãn sau sales demo, nhưng trước public v0.1 nếu giữ PRD hiện tại

- trial tenant cloning/expiry/rate limiting;
- role picker hoàn chỉnh;
- editor permissions theo từng staff/building;
- shift close, cash denominations, gap reason và shift review;
- English, full accessibility/visual regression;
- SSE/reconnect/Last-Event-ID;
- complete metrics/chaos/performance suites.

## 8. Delivery plan đề xuất

### Phase A — unblock UI

1. Cập nhật tracking để phản ánh Git.
2. Đưa Claude Design raw UI source vào repository hoặc xác định vị trí chính thức của nó.
3. Hoàn tất web foundation tối thiểu: route shell, tokens, API client, mock toggle, Vietnamese strings.
4. Không xây component system riêng trước các màn demo.

### Phase B — sales vertical slice

1. Render S1 room map từ API/mock.
2. Bind S2 check-in UI vào API hiện có.
3. Bind S3 stay detail, extras, checkout UI vào API hiện có.
4. Implement payment creation/QR (SG-301).
5. Implement simulator settlement but dùng polling thay SSE cho UI (một version rút gọn của SG-302/303).
6. Implement housekeeping và owner overview mỏng, đủ cho walkthrough.
7. Deploy một environment demo, kiểm tra bằng walkthrough thực tế trên mobile/desktop.

### Phase C — public PRD demo

1. Permissions và shift close.
2. Trial lifecycle, role picker, English.
3. Các test release-only: isolation matrix, five E2E journeys, a11y/visual checks, performance budgets.
4. Public repository checklist, video và release tag.

## 9. Test/CI profile nên dùng

### Mỗi thay đổi

- Go build/test cho packages bị ảnh hưởng;
- pricing golden/property tests nếu đụng pricing;
- unit/integration test cho payment, checkout, idempotency, tenant/access nếu đụng các phần đó;
- web typecheck/build/lint khi dependencies đã được cài;
- gitleaks và dependency/source lockfile hygiene.

### Trước demo hoặc release

- một smoke journey end-to-end trên real stack;
- manual mobile walkthrough;
- check QR payload bằng ngân hàng thật nếu dùng account thật;
- manual check roles/view-only và tenant separation;
- manual visual QA Vietnamese, sau đó English cho public release.

### Có thể chạy nightly/không block feature UI

- Testcontainers full suite;
- mutation review;
- chaos/fault injection;
- Lighthouse/performance benchmark;
- exhaustive cross-tenant operation matrix;
- visual regression hai ngôn ngữ.

## 10. Quyết định cần owner xác nhận

1. Sales demo có bắt buộc English hay chỉ Vietnamese?
2. Sales demo có cần khách tự tạo trial/đổi role hay một demo tenant preset là đủ?
3. Shift reconciliation và permission editor có phải requirement cho cuộc demo đầu tiên, hay chỉ cần owner overview thể hiện loss controls?
4. Claude Design source nằm ở đâu và có thể được đưa vào repository hay không?
5. “Production app” ở giai đoạn này nghĩa là demo sử dụng sample data, hay nhận dữ liệu khách/khách thật? Nếu có dữ liệu thật, không được cắt PII, isolation, idempotency hoặc security checks.

## 11. Final recommendation

Không đổi framework, không bỏ các non-negotiables về tiền/tenant/payment, và không tạo một codebase “split-brain” (nửa strict hexagonal, nửa handler logic không test). Hãy dùng nền backend đã đầu tư như một accelerator, dồn lực vào UI và một vertical sales flow hoàn chỉnh.

Tốc độ thực tế sẽ đến từ:

- chốt scope sales demo khác public v0.1;
- reuse UI có sẵn;
- ship theo user journey thay vì theo layer/story silo;
- hạ ceremony và release-only quality gates;
- polling thay SSE;
- trì hoãn self-service trial, full bilingual, permissions/shift editor khi không cần cho buổi sales đầu.

Đó là con đường nhanh hơn mà vẫn giữ được điều khiến StayGuard khác một CRUD guesthouse app: khách nhìn thấy tiền được tính đúng, chuyển về đúng chủ, và không thể bị staff đánh dấu paid thủ công.
