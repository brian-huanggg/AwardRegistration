# ESG Idea Contest 面試準備

> 初步的題型與方向分類。需求來源：[ESG.md](./ESG.md)。

## 等級定義

| Level | 問題型態 | 期待的回答 |
| --- | --- | --- |
| L1 Junior | 是什麼、怎麼做 | 概念正確，說得出自己做了什麼 |
| L2 Mid | 為什麼這樣做 | 比較替代方案，知道邊界情況 |
| L3 Senior | 如果……會怎樣 | 故障模式、並發、規模，能設計驗證方式 |
| L4 Staff | 要不要做、值不值得 | 取捨、成本、演進路線、對團隊的影響 |

欄位說明：

- **Recommended Answer**：回答的骨架，面試時再補上自己的實作細節與數字。
- **Notice**：容易答錯、或面試官會拿來挑戰的地方。
- **Follow-up**：答完後最可能的下一題。

## Account & Registration

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 註冊時要檢查什麼？ | 工號格式（6 碼數字）、姓名長度、密碼長度。工號由 `UNIQUE` 約束保證不重複。 | 前端驗證只為體驗，以後端為準。 | 工號重複時回什麼？ |
| L2 | 沒有 Email 也沒有 SSO，如何防止有人用同事的工號註冊？ | 單靠系統做不到身分驗證。可選：從 HR 或 Vault 匯入允許註冊的工號名單、Admin 審核後才啟用、或接受風險並以稽核追蹤。 | 這是規格最大的漏洞，要主動提出並說明你的選擇。 | 被冒名註冊後，本人要怎麼取回帳號？ |
| L2 | 忘記密碼怎麼辦？ | 沒有 Email，只能由 Admin 重設：產生臨時密碼、下次登入強制變更、註銷該使用者所有 session，並寫稽核。 | Admin 如何確認提出要求的是本人，屬於流程問題，系統要留下紀錄。 | Admin 能不能看到使用者的密碼？ |
| L2 | 兩人同時用同一工號註冊？ | `UNIQUE (employee_no)` 讓第二筆失敗，攔截錯誤碼 1062。 | 先查再寫有競態條件。 | 回應要不要透露「工號已註冊」？ |
| L2 | 註冊 API 本身會不會被拿來枚舉工號？ | 會，「已註冊」就洩漏帳號存在。對註冊限流並記入稽核；內部系統可評估後接受這個風險。 | 要和登入的防枚舉一起評估，否則登入防住了、註冊卻洩漏。 | 怎麼統一錯誤訊息又不讓使用者困惑？ |
| L2 | 密碼規則怎麼定？ | 依 NIST SP 800-63B：設最短長度、最長 72 bytes（bcrypt 上限）、比對外洩密碼清單；不強制字元組合，也不強制定期更換。 | 長度以 bytes 計算，一個中文字在 UTF-8 佔 3 bytes。 | 已存在的弱密碼怎麼處理？ |
| L3 | 登入、登出、改密碼時 session 怎麼處理？ | 登入一律產生新 token（防 session fixation）；登出刪除 server 端那一列；改密碼或被重設時刪除該使用者所有 session。 | 只清 cookie 不刪 DB 列，被偷走的 token 仍可使用。 | 要不要限制同時登入的 session 數？ |
| L3 | 員工離職後帳號怎麼處理？ | 沒有 HR 同步，帳號不會自動失效。提供 Admin 停用（`disabled_at`），停用時刪除所有 session，並定義其提案與團隊成員關係的處理方式。 | 停用而非刪除，稽核紀錄才能保留對應的操作者。 | 離職者是 Leader 時，提案歸誰？ |
| L4 | 自行註冊的設計值得保留嗎？ | 規格選擇它是為了不依賴 Email 與 SSO、快速上線，代價是身分無法驗證。公司若有 SSO，應作為優先演進方向。 | 要能說出在什麼條件下你會反對這個需求。 | 過渡期兩種登入方式怎麼並存？ |

## Auth & Session

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | Session 和 Cookie 的關係？Cookie 要設哪些屬性？ | Cookie 只帶隨機 session token，狀態存在 server 端 DB。設 `HttpOnly`（防 JS 讀取）、`Secure`（只走 HTTPS）、`SameSite=Lax` 或 `Strict`（降低 CSRF）。 | `SameSite` 不能完全取代 CSRF token。 | 登出時 server 端要做什麼？ |
| L1 | 密碼為什麼用 bcrypt？cost 10 代表什麼？ | bcrypt 是刻意變慢且自帶 salt 的雜湊，cost 10 表示 2^10 輪，單次約數十毫秒，讓離線暴力破解變貴。 | bcrypt 只取前 72 bytes，長密碼要限制長度或先做前處理。 | cost 要怎麼隨硬體調整？ |
| L2 | Session 為什麼只存 SHA-256，而不是 bcrypt？ | token 是 256-bit 隨機值，熵夠高，不可能被暴力反推，快速雜湊就足夠。bcrypt 帶隨機 salt，無法用 hash 直接查索引，每個請求都會變慢。 | 要講清楚「密碼熵低所以用慢雜湊，token 熵高所以用快雜湊」。 | DB 外洩時，攻擊者拿到 hash 能做什麼？ |
| L2 | 閒置 30 分鐘、最長 8 小時怎麼實作？ | session 表存 `created_at`、`last_seen_at`。驗證時用 DB 時間比較兩個條件，任一超過就失效；通過則更新 `last_seen_at`。 | 每個請求都更新會造成大量寫入，可改為距上次更新超過 1 分鐘才寫。 | 這個節流會讓閒置逾時誤差多少？ |
| L2 | 帳號鎖定和 IP throttle 怎麼實作？ | 失敗次數存 DB，用原子 `UPDATE ... SET count = count + 1` 累加，達門檻寫入 `locked_until`。IP 用 15 分鐘的時間窗計數。 | 公司內網常透過 NAT 或 proxy，大量員工共用同一個 IP；要確認 `X-Forwarded-For` 只信任 Ingress 寫入的值。 | 攻擊者故意打錯密碼鎖住別人怎麼辦？ |
| L3 | 計數器和 session 都放在 Galera 上會有什麼問題？ | 同一列被多節點同時更新會產生 certification conflict，commit 時回 deadlock 錯誤（1213），需要重試。可以讓寫入集中到單一節點來避免。 | Galera 的 `SELECT ... FOR UPDATE` 鎖不跨節點，衝突只在 commit 時才發現。 | 為什麼不用 Redis 存 session 和計數器？ |
| L3 | Vault 移除某位 Admin 後，他已登入的 session 怎麼處理？ | 同步時把角色變更寫進 DB，同時刪除或註銷該使用者所有 session；權限檢查每次都讀 DB 的角色，不快取在 session 裡。 | 同步是啟動時做還是定期做，延遲多久，要說得出來。 | 同步失敗時 Admin 名單會停在什麼狀態？ |
| L3 | `last_seen_at` 節流後，在 Galera 上的寫入成本？ | 驗證 session 是一次讀取；節流後每個 session 每分鐘最多一次依主鍵的 `UPDATE`，不同 session 互不衝突，certification conflict 機率低。 | 同一使用者多個分頁同時請求，可能同時更新同一列，要能重試 1213 或直接忽略這次更新失敗。 | session 驗證這次讀取會不會成為列表 API 的瓶頸？ |
| L3 | 分散式 credential stuffing：1000 個 IP、每個帳號只試一次，怎麼辦？ | 帳號鎖定與 IP throttle 都擋不住。需要全域失敗率的監控與告警、比對外洩密碼清單，長期導入 SSO 或 MFA。 | 要承認單一機制擋不住，說出偵測與回應方式。 | 怎麼定義「失敗率異常」？ |
| L3 | 計數器存在 DB，攻擊流量會不會變成 DB 負載？ | 會。throttle 檢查放在 bcrypt 之前，且只是單列讀寫。可在每個 Pod 加記憶體內的前置限流當粗篩，以 DB 計數器為準。 | 記憶體內的限流各 Pod 分開計數，不精確，只能當第一道過濾。 | 為什麼不直接用 Ingress 的 rate limit？ |
| L4 | 之後要接公司 SSO（OIDC），現在的設計要怎麼演進？ | 帳號以工號為主鍵、不依賴 Email，可直接對應 IdP 的員工 ID。登入流程換成 OIDC，session 機制保留；自行註冊與密碼逐步淘汰。 | 過渡期兩種登入並存，要定義帳號合併規則。 | 鎖定與 throttle 的責任移到哪一層？ |

## Authorization & Anti-enumeration

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 401 和 403 差在哪？ | 401 是沒有或無效的身分；403 是知道你是誰但不允許。 | 這個專案在「看不到」的情況改回 404。 | 那什麼時候回 404？ |
| L2 | 為什麼無權限一律回 404？ | 回 403 等於告訴對方「這個資源存在」，可被用來枚舉提案 ID。回 404 讓「不存在」和「沒權限」無法區分。 | 查詢時把擁有者條件放進 `WHERE`，一次查詢就同時處理存在與權限，避免兩次查詢產生時間差。 | 那為什麼檢查順序裡還有 403？ |
| L2 | 檢查順序 401 → CSRF → 404 → 403 → 428/412 → 403 → 422 的理由？ | 先確認身分與請求來源，再確認能不能「看見」，看得見才告訴你不能寫（role 403）。接著檢查前置條件，再檢查截止狀態，最後才驗內容。越早的檢查洩漏越少資訊。 | Member 看得到提案但不能編輯，所以回 403 而非 404。Admin 略過 state 403。 | 為什麼 428/412 排在 state 403 前面？ |
| L2 | dummy bcrypt 解決什麼問題？ | 帳號不存在時直接回錯會比較快，攻擊者用回應時間就能判斷工號是否存在。不存在時仍對一個固定 hash 執行 bcrypt，讓時間一致。 | 鎖定中的帳號也要走同樣路徑，否則可從時間差推出被鎖定。 | 其他 API 的回應時間也一致嗎？ |
| L3 | Leader 用工號加組員，這個功能本身能被拿來枚舉工號嗎？ | 能。加入不存在的工號與存在的工號若回應不同，就洩漏帳號清單。可統一錯誤訊息、對此 API 限流並記入稽核。 | 也可以接受：內部系統的工號本來就不是秘密，要說明風險評估。 | 一個人能不能同時在多個團隊？ |
| L3 | 資源層級授權怎麼實作，才不會漏掉某個 endpoint？ | 授權集中在 proposal 模組的 repository 查詢（例如 `FindVisible(ctx, actor, id)`），handler 拿不到「不帶授權」的查詢方法。另外寫表格驅動測試，涵蓋每個角色對每個 endpoint。 | 下載 API 也要走相同的可見性檢查。 | Admin 同時是提案者時，權限怎麼切開？ |
| L3 | 除了狀態碼，還有哪些側通道會洩漏資源存在？ | 回應時間、`Content-Length`、ETag、錯誤訊息、列表總數。把可見性條件放進同一句查詢的 `WHERE`，404 的 body 固定不變。 | dummy bcrypt 只處理登入；資源 API 靠「一次查詢」維持時間一致。 | 怎麼測試兩種 404 的 body 完全相同？ |
| L3 | CSRF 檢查排在 404 前面，會不會洩漏資訊？ | 不會。CSRF 驗證只看 session 與 token，和資源無關；排在前面反而讓未通過的請求碰不到資源查詢。 | 原則是「先確認身分與來源，再處理資源」。 | GET 也要檢查 CSRF 嗎？ |
| L4 | 規則變多時（評審、多階段審查），授權模型要怎麼演進？ | 從寫死的角色判斷，演進到集中的 policy 函式，必要時再導入 ReBAC（例如 OpenFGA）。這個規模先維持程式內的 policy。 | 不要為了未來需求先引入外部授權服務。 | 如何稽核授權規則的變更？ |

## Permission Matrix

> 假設請求已通過認證與 CSRF 檢查、`If-Match` 正確、內容合法，只看角色與狀態。未登入一律 401。「待確認」是規格沒有定義的格子，要和需求方確認後寫回規格。

角色定義：

- **無關使用者**：已登入，但不是這份提案的 Leader 或 Member。
- **Admin**：與這份提案無關的 Admin。
- **Admin 兼 Leader**：Admin 同時是這份提案的 Leader。截止後是否仍享有豁免屬於政策決定（見 Admin 節）。

### 全域動作

| 動作 | 一般使用者 | Admin |
| --- | --- | --- |
| 列表提案 | 200，只含自己是 Leader 或 Member 的提案 | 200，全部提案 |
| 建立提案（自己成為 Leader） | OPEN：201；CLOSED：403；NOT OPEN：待確認 | 201，不受狀態影響 |
| 替他人建立提案 | 404 | 待確認（規格「替管理員建立編輯簡報」語意不明） |
| 設定截止時間 | 404 | 200 |
| 查詢 Audit Log | 404 | 200 |
| 統計儀表板、Excel 匯出 | 404 | 200 |

### 資源層級動作（針對某一份提案）

| 動作 | 無關使用者 | Member | Leader | Admin | Admin 兼 Leader |
| --- | --- | --- | --- | --- | --- |
| 檢視詳情 | 404 | 200 | 200 | 200 | 200 |
| 下載簡報 | 404 | 200 | 200 | 200 | 200 |
| 整筆替換（含簡報） | 404 | 403 | 200，截止後 403 | 200 | 200，截止後待確認 |
| 刪除提案 | 404 | 403 | 204，截止後 403 | 204 | 204，截止後待確認 |
| 加入組員 | 404 | 403 | 201，截止後 403 | 201 | 201，截止後待確認 |
| 移除組員 | 404 | 403 | 待確認 | 待確認 | 待確認 |
| 自行退出團隊 | 404 | 403 | — | — | — |

### 題目

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 為什麼 Member 編輯回 403，無關使用者卻回 404？ | Member 看得見這份提案，告訴他「不能寫」不會洩漏任何資訊；無關使用者看不見，回 404 讓「不存在」和「沒權限」無法區分。 | 判斷依據是「能不能看見」，不是「有沒有權限」。 | 那 Admin API 對一般使用者回什麼？ |
| L2 | 一般使用者呼叫 Admin API，回 404 還是 403？ | Admin API 的路徑寫在前端程式碼裡，本來就不是秘密，回 403 也合理。一律回 404 的好處是規則單純、不必逐一判斷。 | 選定之後寫進規格，所有 Admin API 一致。 | 前端要不要隱藏 Admin 選單？ |
| L2 | Member 在截止後嘗試編輯，回哪一種 403？ | role 403 排在 state 403 之前，所以回 role 403。 | 兩種 403 要用不同的錯誤碼區分（見 API Error Contract），否則前端無法顯示正確的訊息。 | 前端怎麼依錯誤碼顯示訊息？ |
| L2 | 畫這張矩陣時發現哪些規格缺口？ | NOT OPEN 能不能建立提案、Leader 能不能移除組員、Admin 兼 Leader 截止後是否豁免、「替管理員建立」的語意。 | 矩陣的價值在於逼出缺口，每一格都要有答案。 | 需求方不回應時怎麼辦？ |
| L3 | 怎麼保證實作和矩陣一致？ | 把矩陣本身當成表格驅動測試的資料：角色 × 動作 × 狀態，每一格一個 test case，斷言狀態碼與錯誤碼。新增 endpoint 時若矩陣裡沒有對應的列，就讓測試失敗。 | 要用真的 DB 建立各角色的資料，不要 mock 授權邏輯。 | 矩陣變大後，測試時間怎麼控制？ |
| L3 | 列表 API 的可見範圍怎麼和矩陣一致？ | `WHERE (p.leader_id = ? OR EXISTS (SELECT 1 FROM proposal_members m WHERE m.proposal_id = p.id AND m.account_id = ?))`，使用者的篩選條件以 `AND` 接在可見範圍之後。 | 漏了括號時，`a OR b AND c` 會先算 `AND`，篩選條件可能讓人看到別人的提案。 | 這個查詢要建哪些索引？ |
| L3 | 矩陣在程式裡怎麼表達？ | 集中成一個 policy 函式，例如 `Decide(actor, action, proposal, state)`，回傳 Allow、NotFound、Forbidden、Closed；handler 只負責把結果轉成狀態碼。 | 不要讓每個 handler 各自判斷，漏改一處就是越權。 | Admin 的豁免寫在哪一層？ |
| L4 | 矩陣要在哪裡維護？ | 只能有一個來源：規格文件、程式與測試資料三者之一為準，其他從它產生或比對。建議以測試資料為準，再產生文件。 | 文件和程式各自維護，遲早會不一致。 | 規則變更要怎麼稽核？ |

## API Error Contract

錯誤回應採用 RFC 9457 Problem Details（`application/problem+json`），另外擴充機器可讀的 `code` 與 `request_id`。

```http
HTTP/1.1 409 Conflict
Content-Type: application/problem+json

{
  "type": "https://esg.example/errors/proposal-title-duplicate",
  "title": "Proposal title already exists",
  "status": 409,
  "code": "PROPOSAL_TITLE_DUPLICATE",
  "detail": "The leader already has a proposal with the same title.",
  "instance": "/api/proposals",
  "request_id": "0f8c2a5e-6b1d-4f7e-9a3c-2d4b5e6f7a8b"
}
```

422 用 `errors[]` 帶出逐欄位的錯誤，讓前端自行組成訊息：

```json
{
  "status": 422,
  "code": "VALIDATION_FAILED",
  "errors": [
    { "field": "title", "code": "TOO_LONG", "max": 100 },
    { "field": "slides", "code": "FILE_TYPE_INVALID" }
  ]
}
```

### 錯誤碼對照

| 狀態碼 | code | 情境 |
| --- | --- | --- |
| 401 | `UNAUTHENTICATED` | 沒有 session，或 session 已逾時 |
| 401 | `INVALID_CREDENTIALS` | 登入失敗，帳號不存在、密碼錯誤、帳號鎖定都回這個 |
| 403 | `CSRF_INVALID` | CSRF token 缺少或不符 |
| 404 | `NOT_FOUND` | 不存在或看不見，body 固定不變 |
| 403 | `FORBIDDEN_ROLE` | 看得見但角色不允許，例如 Member 編輯 |
| 428 | `PRECONDITION_REQUIRED` | 寫入時沒帶 `If-Match` |
| 412 | `VERSION_MISMATCH` | `If-Match` 與目前版本不符 |
| 403 | `CONTEST_NOT_OPEN`、`CONTEST_CLOSED` | 截止狀態不允許寫入 |
| 422 | `VALIDATION_FAILED` | 內容不合法，細節在 `errors[]` |
| 409 | `PROPOSAL_TITLE_DUPLICATE`、`EMPLOYEE_NO_TAKEN`、`TEAM_FULL`、`MEMBER_ALREADY_ADDED` | 與現有資料衝突 |
| 413 | `PAYLOAD_TOO_LARGE` | 超過大小上限 |
| 429 | `TOO_MANY_ATTEMPTS` | IP throttle，帶 `Retry-After` |
| 503 | `STORAGE_UNAVAILABLE` | MinIO 無法使用，帶 `Retry-After` |
| 500 | `INTERNAL_ERROR` | 未預期的錯誤，只給 `request_id` |

### 題目

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 錯誤回應的格式是什麼？ | RFC 9457 Problem Details：`type`、`title`、`status`、`detail`、`instance`，再擴充 `code` 與 `request_id`。 | 所有錯誤都用同一個格式，包括框架自動產生的 400、404、405。 | 為什麼要額外的 `code`？ |
| L2 | `code` 和 `message` 為什麼分開？ | `code` 給程式判斷與 i18n，是穩定的契約；`title`、`detail` 給開發者閱讀，可以修改。前端依 `code` 決定顯示的文字與行為。 | 前端不能用字串比對 `detail`。 | 錯誤訊息要做多語系嗎？ |
| L2 | 錯誤碼要寫進 OpenAPI 嗎？ | 要。每個 endpoint 列出可能的狀態碼與 `code`。改名是 breaking change；新增 `code` 時，前端要有處理未知 `code` 的預設行為。 | 只列狀態碼不夠，兩種 403 的處理方式不同。 | 怎麼在 CI 檢查實際回應符合規格？ |
| L2 | 404 的 body 怎麼確保一致？ | 用一個固定的常數回應，不依情境改變 `detail`，`Content-Length` 也就相同。 | 「不存在」和「沒權限」只要有一個 byte 不同，就能被用來枚舉。 | 怎麼測試兩種 404 完全相同？ |
| L2 | 409、412、422 的邊界在哪？ | 422 是內容本身不合法，不需要查 DB 就能判斷；409 是和現有資料衝突；412 是前置條件（版本）不符。 | 「名稱重複」需要查 DB，是 409 不是 422。 | 「團隊已滿」算哪一種？ |
| L3 | 1062 怎麼對應到不同的錯誤碼？ | MariaDB 的錯誤訊息包含索引名稱，例如 `Duplicate entry '...' for key 'uq_proposals_leader_title'`。約束要明確命名，再依名稱對應 `code`。 | 原始錯誤訊息含使用者資料與 schema 資訊，不能回給客戶端。 | 為什麼不在寫入前先查一次？ |
| L3 | Ingress 回的 413、502、504 不是 problem+json，前端怎麼辦？ | 前端要能處理非 JSON 的錯誤回應，依狀態碼顯示通用訊息。也可以在 Ingress 設定自訂錯誤頁，回傳 JSON。 | 5 MiB 的上傳最容易先被 Nginx 以 413 擋下，後端根本收不到。 | Ingress 的 body 上限要設多少？ |
| L3 | 錯誤細節在回應與日誌之間怎麼分？ | 回應只給 `code` 與 `request_id`；stack trace、SQL 錯誤只寫進日誌。使用者回報 `request_id`，就能查到完整紀錄。 | 500 的 `detail` 不能帶例外訊息。 | 日誌要怎麼依 `request_id` 查詢？ |
| L3 | 登入失敗為什麼都回 `INVALID_CREDENTIALS`？ | 若帳號鎖定時回不同的錯誤碼，攻擊者就能確認帳號存在。 | 使用者會困惑，可在訊息中統一說明「帳號或密碼錯誤，或帳號暫時鎖定」。 | IP throttle 為什麼可以回 429？ |
| L4 | 錯誤碼怎麼治理？ | 命名規則（`DOMAIN_REASON`）、集中定義在一個檔案、變更要經過 review、淘汰時先標記 deprecated 再移除。 | 錯誤碼一旦被前端或其他系統依賴，就很難改。 | 拆成多個服務後，錯誤碼要怎麼統一？ |

## Proposal & Concurrency

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | ETag 和 If-Match 是什麼？ | 讀取時回傳代表版本的 ETag；更新時用 `If-Match` 帶回，版本不符就拒絕。缺少 `If-Match` 回 428，版本不符回 412。 | 這個專案強制要帶，所以缺少時回 428，不是直接放行。 | ETag 是怎麼產生的？ |
| L2 | 為什麼用樂觀鎖而不是悲觀鎖？ | 衝突機率低，且使用者編輯時間長，不能在使用者思考期間持有 DB 鎖。樂觀鎖把衝突留到寫入那一刻再判斷。 | 悲觀鎖跨 HTTP 請求根本做不到。 | SQL 怎麼寫？ |
| L2 | 樂觀鎖的 SQL 怎麼寫？影響 0 列怎麼判斷原因？ | `UPDATE proposals SET ..., version = version + 1 WHERE id = ? AND version = ?`。影響 0 列時再查一次：查不到（或看不到）回 404，查得到代表版本不符，回 412。 | MariaDB 預設回傳「實際改變的列數」，資料相同時會回 0。Go driver 要設 `clientFoundRows=true`。 | 整筆替換時，如何把資料和檔案一起換掉？ |
| L2 | 同一 Leader 的提案名稱不可重複，怎麼實作？ | 複合 `UNIQUE (leader_id, title)`，交給 DB 保證。違反時攔截錯誤碼 1062，轉成 409。 | 先查再寫有競態條件，只有約束可靠。要注意 collation 的大小寫與尾端空白比較規則。 | 為什麼不在應用層檢查就好？ |
| L3 | Galera 多節點同時更新同一筆提案，樂觀鎖還正確嗎？ | 正確。兩個交易在不同節點都通過 `WHERE version = ?`，但 commit 時 Galera certification 會讓後者失敗（回 deadlock 錯誤）。重試時版本已變，最後得到 412。 | 要把 certification 失敗和 412 分開處理：前者可重試，後者要回給使用者。 | 重試幾次？要不要退避？ |
| L3 | multipart 建立提案怎麼保證原子性？ | 先用新的 object key 上傳 MinIO，成功後在單一 DB 交易內寫入提案、檔案中繼資料與稽核。DB 失敗時物件成為孤兒，由排程比對 MinIO 與 DB 後清除。 | DB 和 MinIO 不可能有分散式交易，要講「最終一致＋清理」，不要說「完全原子」。 | 為什麼不先寫 DB 再上傳？ |
| L3 | ETag 用 strong 還是 weak？內容是什麼？ | strong，由 `version` 產生並加上引號，例如 `"7"`。`If-Match` 依 RFC 9110 使用強比較，weak ETag（`W/` 開頭）永遠不符。 | 不要用 `updated_at` 當版本，秒級精度在同一秒內的兩次更新會相同。 | 列表 API 要回 ETag 嗎？ |
| L3 | 整筆替換時，新檔已上傳 MinIO，DB 更新卻失敗（412 或 500）？ | DB rollback，提案仍指向舊物件，新物件成為孤兒由排程清除。使用者收到 412 或 500，系統沒有半套狀態。 | 可在上傳前先比對版本，減少白傳的檔案；但最終仍以交易內的 `WHERE version = ?` 為準。 | 先驗版本再上傳，還需要交易內的檢查嗎？ |
| L3 | 標題唯一性遇到大小寫、尾端空白、Unicode 正規化？ | 大小寫由 collation 決定（`_ci` 視為相同）；PAD SPACE 的 collation 比較時忽略尾端空白；NFC 與 NFD 要在應用層統一正規化成 NFC 並 trim。 | 要在規格中定義「相同標題」的意思。 | 顯示時保留使用者的原始輸入嗎？ |
| L3 | 使用者連點兩次送出？ | 第二筆被 `UNIQUE (leader_id, title)` 擋下回 409。若要真正的冪等，可加 `Idempotency-Key` header。 | 收到 409 時，前端要能引導使用者找到已建立的那筆。 | Idempotency-Key 存哪裡、存多久？ |
| L4 | 整筆替換、不支援部分更新，代價是什麼？ | 前端每次都要送完整資料與檔案，頻寬較大；但 API 語意單純、樂觀鎖容易做對。這個規模接受這個代價。 | 若之後要支援只改文字，可新增 `PATCH` 並沿用同一個 version。 | 什麼條件下你會改設計？ |

## Team & Membership

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | Leader 怎麼加入組員？ | 用工號找到帳號，寫入成員表 `(proposal_id, member_id)`，以複合主鍵防止重複加入。 | 加入 Leader 本人要擋下。 | 工號不存在時回什麼？ |
| L2 | 「最多 4 位 Member」怎麼讓 DB 保證？ | `CHECK` 無法計算列數。可用 `slot TINYINT CHECK (slot BETWEEN 1 AND 4)` 加 `UNIQUE (proposal_id, slot)`，第五人找不到空位就失敗。或先 `SELECT ... FOR UPDATE` 鎖住提案列再計數。 | Galera 的 `FOR UPDATE` 鎖不跨節點，slot 方案由唯一約束保證，較可靠。 | 移除成員後 slot 怎麼回收？ |
| L2 | 一個人能否同時在多個團隊？Leader 能否是別隊的 Member？ | 規格未定義，要先決定並寫進規格。若一人只能在一隊，用 `UNIQUE (member_id)`，另外還要確認他不是任何提案的 Leader。 | 跨兩張表的唯一性無法用單一約束保證，要說明怎麼補。 | 為什麼這條規則難以用約束表達？ |
| L2 | 成員不能自行退出，那 Leader 能移除成員嗎？能轉移 Leader 嗎？ | 規格未說明，列為待確認。若允許，移除與轉移都要寫稽核、截止後擋下。轉移 Leader 會影響 `UNIQUE (leader_id, title)`。 | 主動列出規格缺口，比自行假設答案更好。 | 轉移後原 Leader 變成什麼角色？ |
| L3 | 成員異動算不算提案的寫入？要不要遞增 version？ | 若成員是提案資源的一部分，就要遞增 version、受截止限制、寫稽核。否則另一位編輯者可能依據舊的成員名單覆寫。 | 整筆替換的 API 若包含成員清單，語意要和獨立的成員 API 一致。 | 成員 API 也要帶 If-Match 嗎？ |
| L3 | 成員被移除時，他正在下載簡報？ | 授權在每個請求開始時檢查，進行中的串流會完成；下一個請求回 404。 | 要能說清楚「授權在什麼時間點檢查」。 | 需要中斷進行中的下載嗎？ |

## Deadline & Clock

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 狀態 NOT OPEN / OPEN / CLOSED 怎麼算？ | 不存狀態欄位，每次用 DB 現在時間和開始、截止時間比較即時推算。 | 存欄位需要排程去切換，切換延遲就會出錯。 | 為什麼用 DB 時間？ |
| L2 | 為什麼唯一時鐘是 DB 時間？ | 多個 Pod 的系統時間可能有偏差，以 DB 為準讓所有判斷一致。連線設定 `time_zone = '+00:00'`，用 `UTC_TIMESTAMP()`。 | `NOW()` 會受 session 時區影響。 | Galera 各節點的時鐘有偏差怎麼辦？ |
| L3 | 截止前一秒送出的請求，會不會在截止後才寫入？ | 若先查狀態、再寫入，就有 TOCTOU。把截止條件放進同一個交易或同一句 SQL（`WHERE` 內比較時間），以寫入當下的 DB 時間為準。 | 要定義「以 commit 時間為準」還是「以請求到達時間為準」，並寫進規格。 | 大檔案上傳花了 10 秒，跨過截止怎麼算？ |
| L3 | Galera 節點時鐘不一致的影響？ | 時間函式在執行的節點求值，各節點要用 NTP 同步。偏差在毫秒級時，對分鐘級的截止影響可忽略。 | 不要宣稱「DB 時間保證全域一致」。 | 如何監控時鐘偏差？ |
| L3 | 上傳花了 10 秒，開始時未截止、commit 時已截止，怎麼算？ | 建議以 commit 當下的 DB 時間為準，規則單純，也無法靠拖延上傳繞過。代價是截止前開始的上傳會失敗，前端要提醒提早送出。也可以訂寬限期，但要寫進規格並一致套用。 | 若以請求到達時間為準，要由後端記錄到達時間，不能信任客戶端。 | 寬限期要多長？ |
| L4 | Admin 不受截止影響，這個規則有什麼風險？ | Admin 可在截止後修改他人提案，影響競賽公平性。需要稽核紀錄記下操作者與「略過截止」，並在儀表板呈現。 | Admin 本身也可能是提案者，存在利益衝突。 | 要不要禁止 Admin 修改自己團隊的提案？ |

## Admin

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L2 | 截止時間設定怎麼驗證與儲存？ | 單列設定表（`id = 1` 加 `CHECK`），並加 `CHECK (opens_at < closes_at)`。變更寫稽核，記錄新舊值。 | 是否允許設為過去時間、是否允許重新開放，要寫進規格。 | 為什麼不放在 ConfigMap 或環境變數？ |
| L3 | Admin 把截止時間提前，使用者正在編輯？ | 寫入在 commit 時以 DB 時間判斷，送出時回 state 403。前端保留草稿並說明原因。 | 截止時間是全域設定，變更會立即影響所有人。 | 要不要通知使用者截止時間變更？ |
| L3 | Admin 代他人建立或編輯簡報，稽核怎麼記？ | 同時記 `actor_id`（實際操作者）與 `owner_id`（提案擁有者），並標記是否略過截止。不能讓紀錄看起來像擁有者本人操作。 | 規格寫「替管理員建立編輯簡報」語意不明，要確認是「替提案者」還是「替其他管理員」。 | 被代為操作的使用者要不要收到通知？ |
| L3 | Admin 查詢 Audit Log 的 API 要注意什麼？ | 強制時間範圍與分頁上限、篩選欄位用白名單、查詢要走索引。查詢稽核這個動作本身也寫稽核。 | 稽核表會是最大的表，不加限制的查詢會拖垮 DB。 | 要不要支援全文搜尋？ |
| L4 | Admin 本身是提案者，截止後能改自己的提案嗎？ | 這是政策決定。建議 Admin 對自己參與的提案不享有截止豁免，或需要另一位 Admin 核准。 | 技術上做得到，重點是公平性與可稽核。 | 怎麼判斷「自己參與」的提案？ |

## Slides & Storage

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 5 MiB 上限在哪裡擋？ | 多層：Ingress / Nginx 的 `client_max_body_size`，後端用 `http.MaxBytesReader` 包住 body。 | 只看 `Content-Length` 不夠，可以被偽造或省略。 | multipart 的其他欄位也算在 5 MiB 內嗎？ |
| L2 | 怎麼確認上傳的是 PDF 或 PPTX？ | 不信副檔名和客戶端的 `Content-Type`。看檔頭 magic bytes：PDF 是 `%PDF-`，PPTX 是 zip（`PK\x03\x04`），並檢查 `[Content_Types].xml` 含 presentationml。 | PPTX 是 zip，要防 zip bomb：限制解壓大小與項目數，或只讀目錄不解壓。 | 要不要掃毒？ |
| L2 | 為什麼每次上傳都用新的 object key？ | 覆蓋同一個 key 時，正在下載的人可能讀到混合內容；交易失敗也無法還原。新 key 讓 DB 指向哪個物件就是哪個版本，舊物件寫進 `pending_file_deletion` 稍後清除。 | 舊物件的刪除登記要和資料異動在同一個交易。 | 清除排程怎麼避免多個 Pod 重複執行？ |
| L2 | 下載為什麼由後端串流，不用 presigned URL？ | 每次下載都要經過授權與稽核；presigned URL 發出後就無法撤銷，也無法記錄實際下載。後端用 `io.Copy` 從 MinIO 串流到回應，記憶體用量固定。 | 要設 `Content-Disposition`（中文檔名用 RFC 5987 的 `filename*`）與 `X-Content-Type-Options: nosniff`。 | 下載到一半斷線，稽核記成功嗎？ |
| L3 | `pending_file_deletion` 的清除排程在多 replica 下怎麼避免重複？ | 用租約欄位搶工作：`UPDATE ... SET lease_until = 未來時間 WHERE lease_until < UTC_TIMESTAMP() LIMIT n`，搶到的列才處理。刪除本身是冪等的，重複執行也無害。 | MariaDB 10.5 不支援 `SKIP LOCKED`（10.6 才有），Galera 也不複製 `GET_LOCK`。 | 或改用 K8s CronJob 加 `concurrencyPolicy: Forbid`？ |
| L3 | multipart 解析在 distroless 會遇到什麼問題？ | `ParseMultipartForm` 超過記憶體門檻會寫暫存檔；若 root filesystem 唯讀就會失敗。改用 `MultipartReader` 串流處理，或掛 `emptyDir` 到 `/tmp`。 | 串流處理時，要先讀完文字欄位再處理檔案，欄位順序要在 API 規格中約定。 | 如何邊上傳邊驗證檔頭？ |
| L3 | 「有預期檔案名稱」怎麼處理？ | 下載檔名由後端依規則產生（例如提案標題加副檔名），不沿用使用者上傳的原始檔名；移除路徑分隔與控制字元，中文用 `filename*`。object key 也不使用使用者輸入。 | object key 含使用者輸入會帶來路徑穿越與枚舉風險。 | 原始檔名要不要保存？ |
| L3 | magic bytes 驗證通過，但檔案是 PDF polyglot，或其實是帶巨集的簡報？ | 檔頭驗證只能降低風險。下載一律 `attachment` 加 `nosniff`；需要時加上 ClamAV 掃毒或 CDR（內容解除與重建）。 | 帶巨集的是 PPTM，可檢查 zip 內是否有 `vbaProject.bin` 並拒絕。 | 掃毒要同步做還是非同步做？ |
| L3 | 清除排程刪除物件時，剛好有人在下載？ | 進入 `pending_file_deletion` 的物件已不被 DB 參照，新的下載請求拿不到它；進行中的串流則可能被中斷。設最短保留時間（例如 1 小時）再刪即可避免。 | 最短保留時間同時解決備份還原的問題（見 Backup & DR）。 | 保留時間怎麼定？ |
| L4 | 檔案量成長 100 倍，還維持後端串流嗎？ | 頻寬與 Pod 數量會成為瓶頸。可改成後端授權並寫稽核後，回傳短效 presigned URL；代價是無法確認實際下載完成。 | 這是「稽核精確度」和「擴展成本」的取捨，需要和需求方確認。 | 那上傳也改成直傳 MinIO 嗎？ |

## Audit

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 稽核紀錄要記哪些欄位？ | 時間（DB 時間）、操作者、動作、目標資源、結果（成功／拒絕與原因）、來源 IP、request-id。 | 不記密碼、token 等敏感資料。 | 為什麼要記 request-id？ |
| L2 | 「稽核寫入失敗則整筆失敗」怎麼實作？ | 稽核紀錄和業務資料寫在同一個 DB 交易，任一失敗就 rollback，所以不可能有成功但沒稽核的寫入。 | 稽核不能用非同步佇列寫，否則會有「業務已成功、稽核遺失」的窗口。 | 被拒絕的請求怎麼記？ |
| L2 | 被拒絕的嘗試怎麼記？ | 業務交易已 rollback，所以拒絕紀錄要另外開一個交易寫入。寫入失敗時回應仍然是錯誤，不影響結果。 | 遭受攻擊時拒絕紀錄會暴增，要考慮限流或彙總。 | 稽核表會長多大？ |
| L3 | 下載不是 DB 寫入，怎麼保證「沒有漏記的成功下載」？ | 在送出第一個 byte 之前寫入稽核並 commit；稽核失敗就回 500，不送檔案。紀錄語意是「已授權並開始傳送」。 | 客戶端中途斷線無法回寫成功與否，要誠實說明紀錄的語意。 | 要不要在傳送完成後補一筆完成紀錄？ |
| L3 | hash chain 怎麼做？並發寫入時有什麼問題？ | 每筆存 `hash = SHA-256(prev_hash ‖ 正規化的紀錄內容)`。並發時必須序列化取得前一筆 hash，形成熱點；在 Galera 上會變成大量 certification conflict。 | 可改為 commit 後由單一 worker 依 id 順序非同步補算 hash，鏈的計算不必放在業務交易內。 | 非同步補算時，竄改窗口有多大？ |
| L3 | 怎麼防止 DBA 或程式直接改稽核表？ | 應用程式帳號對稽核表只授予 `INSERT` 和 `SELECT`。hash chain 可偵測竄改但無法防止；定期把鏈頭 hash 寫到外部（例如另一個系統或 WORM 儲存）作為錨點。 | 有 DB 管理權限的人可重算整條鏈，所以需要外部錨點。 | 驗證整條鏈要多久？ |
| L3 | 遭受攻擊時拒絕紀錄暴增，會不會反過來拖垮 DB？ | 會。拒絕紀錄可依 IP 與時間窗彙總（記次數），或抽樣；成功的寫入永遠逐筆記錄。 | 「不漏記」的要求針對成功寫入；拒絕紀錄能否彙總，要和需求方確認。 | 彙總後還能調查單一事件嗎？ |
| L4 | 稽核資料的保存與查詢策略？ | 依法遵需求定義保存期限，按月分區（partition）方便歸檔與刪除。查詢走 `(created_at, actor_id)` 等索引並強制分頁。 | 刪除舊資料會斷開 hash chain，要定義歸檔時的鏈錨點。 | 稽核要不要送到 SIEM？ |

## Bonus Features

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L2 | Excel 匯出有什麼安全風險？ | 公式注入（Formula Injection）：開頭為 `=`、`+`、`-`、`@` 的儲存格要跳脫（例如前面加 `'`）。只限 Admin、寫入稽核、檔名由後端產生。 | 提案標題是使用者輸入，最容易被塞入公式。 | 匯出內容含個資要怎麼處理？ |
| L3 | 匯出一萬筆，怎麼不吃光記憶體？ | 用 excelize 的 `StreamWriter` 邊查邊寫；DB 以 keyset 分批讀取，放在唯讀交易內取得一致快照，直接串流到回應。 | 長交易會保留大量 MVCC 歷史版本，要控制匯出時間。 | 要不要改成非同步產生、完成後再下載？ |
| L3 | 統計儀表板即時算還是預先算？ | 一萬筆的規模，有索引的 `GROUP BY` 即時算即可。若預先計算，要定義可接受的延遲與更新時機。 | 先量測再決定，不要一開始就建彙總表。 | 截止前尖峰大量刷新儀表板怎麼辦？ |
| L3 | 孤兒檔案排程怎麼找出「MinIO 有、DB 沒有」的物件？ | 列出 bucket 與 DB 的參照比對。只刪除建立超過 N 小時的物件，避免刪到 DB 交易尚未 commit 的上傳。 | 沒有年齡門檻會產生競態，刪掉正在建立的提案的檔案。 | N 怎麼定？ |
| L3 | Audit hash chain 怎麼驗證？ | 從錨點依 id 順序重算每筆 hash 並比對；可增量驗證，只驗上次驗證點之後的紀錄。 | 正規化格式（欄位順序、時間格式、編碼）一旦變更，舊紀錄就驗不過，格式要版本化。 | 驗證失敗時要做什麼？ |
| L4 | 加分功能要先做哪個？ | 依風險與價值排序：孤兒排程補上一致性缺口，優先；hash chain 提升稽核可信度；儀表板與匯出是使用者看得到的價值。 | 要能說出沒做的項目為什麼可以延後。 | 時間不夠時你會砍哪個？ |

## Fault Tolerance

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | MinIO 掛掉時各 API 的行為？ | 刪除仍成功；上傳與下載回 503；列表與詳情不受影響。 | 503 可附上 `Retry-After`。 | 為什麼刪除可以成功？ |
| L2 | 為什麼刪除不需要 MinIO？ | 刪除只在 DB 交易內刪資料、把 object key 寫進 `pending_file_deletion` 並寫稽核，不呼叫 MinIO。MinIO 恢復後由排程清除。 | 這是把外部呼叫移出關鍵路徑的典型做法。 | 排程清除失敗會怎樣？ |
| L2 | 怎麼判斷 MinIO 掛了？ | 每次呼叫設 context timeout；連線錯誤或逾時轉成 503。可加 circuit breaker，連續失敗後短時間直接回 503，不再等逾時。 | 沒有逾時的話，請求會卡住並耗盡連線。 | timeout 設多少？ |
| L3 | readiness probe 要不要檢查 MinIO？ | 不要。檢查的話 MinIO 一掛，所有 Pod 都被移出 Service，連不需要 MinIO 的功能也失效。readiness 只檢查 DB；MinIO 狀態用 metrics 與告警。 | liveness 更不能依賴外部服務，否則會造成重啟風暴。 | DB 掛掉時呢？ |
| L3 | 怎麼測試 MinIO 中斷？ | 整合測試中用 testcontainers 停止 MinIO 容器，或用 Toxiproxy 注入延遲與斷線，驗證刪除成功、上傳下載回 503、DB 無殘留不一致。 | 要同時測「完全斷線」和「很慢」兩種情況。 | 怎麼測 DB 中斷？ |
| L3 | MinIO 沒掛，只是每次呼叫要 5 秒？ | 比掛掉更危險：請求卡住，佔滿 goroutine 與連線。用 context timeout 與 circuit breaker，並對檔案 I/O 設並發上限（bulkhead），讓列表 API 的 p95 不受影響。 | breaker 開啟期間要定期放少量請求試探（half-open），否則恢復後也不會關閉。 | timeout 與 breaker 門檻怎麼定？ |
| L3 | Galera 某個節點被網路隔離？ | 被隔離的節點成為 non-primary，拒絕查詢（`WSREP has not yet prepared node for application use`）。前面的 proxy（MaxScale、ProxySQL）或連線設定要能繞開它。 | readiness 若檢查 DB，要確認連到的是可用節點，而不是固定某一個。 | 叢集為什麼要奇數個節點？ |
| L3 | Vault 連不上時怎麼辦？ | 啟動時拿不到機密就不啟動（fail fast），讓 K8s 重試。Admin 名單同步失敗時保留上次同步的結果，並發出告警。 | 清空名單會讓所有 Admin 失去權限；保留名單則代表已移除的 Admin 仍有權限。取捨要寫明。 | 同步延遲的上限是多少？ |
| L3 | Galera 失去 quorum 時？ | 剩下的節點成為 non-primary，拒絕讀寫。DB 是核心依賴，API 整體回 503。不要自動 bootstrap，否則可能腦裂。 | 恢復時依 `grastate.dat` 的 `safe_to_bootstrap` 與 seqno 選出要 bootstrap 的節點。 | 誰有權限執行 bootstrap？ |
| L3 | 1213 要怎麼重試？ | 只對 certification conflict 與 deadlock 重試，整個交易重來；用指數退避加 jitter，上限約 3 次。交易內不能有外部副作用，MinIO 上傳已在交易之外。 | 412 不能重試，那是使用者層級的衝突。 | 重試之後仍失敗，回什麼？ |
| L3 | 各層的 timeout 怎麼設？ | 由外往內遞減：Ingress 的 proxy timeout > server 的 timeout > MinIO 呼叫 > DB 查詢。下載與上傳要另外設定。 | 外層先逾時，內層還在處理，會白做工並占用資源。 | 怎麼從日誌看出是哪一層逾時？ |
| L4 | 哪些依賴要做部分可用，哪些直接失敗？ | 依功能對依賴的需求分級：DB 是核心，掛掉就整體不可用；MinIO 只影響檔案；Vault 只影響啟動與同步。依此決定 probe、逾時與告警等級。 | 部分可用增加測試與維運成本，只對值得的依賴做。 | 要不要做 Galera 跨機房？ |

## Backup & DR

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L2 | DB 和 MinIO 怎麼備份？ | DB 用 mariabackup 做全量備份加 binlog，可還原到指定時間點；MinIO 用版本控制或 bucket 複寫。備份要加密，存放在不同位置。 | Galera 從單一節點備份即可，備份期間該節點可能進入 desync。 | 備份多久做一次？ |
| L3 | DB 和 MinIO 無法同時備份，怎麼確保一致？ | 先備 DB 再備 MinIO。object key 不可變、只會新增，所以較新的 MinIO 快照會是 DB 參照物件的超集。 | 前提是清除排程不會刪掉 DB 備份仍參照的物件。 | 順序反過來會怎樣？ |
| L3 | DB 還原到昨天，但清除排程已刪除昨天仍被參照的物件？ | `pending_file_deletion` 的實際刪除要延遲至少一個備份保留期，或開啟 MinIO 版本控制讓刪除可以復原。 | 這是「延遲刪除」除了容錯以外的第二個理由。 | 延遲刪除的儲存成本多少？ |
| L4 | RPO 和 RTO 是多少？做過還原演練嗎？ | 說出實際數字與依據（例如 RPO 取決於 binlog 備份頻率）。沒有演練過的備份不算有備份。 | 截止前後的資料最關鍵，可在截止後立即做一次全量備份。 | 演練時怎麼驗證資料正確？ |

## Observability

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L2 | 要收哪些 metrics、設哪些告警？ | 各路由的 RED（請求率、錯誤率、延遲）；MinIO 錯誤率與 circuit breaker 狀態；Galera `wsrep_local_cert_failures`、`wsrep_cluster_size`；帳號鎖定次數；稽核寫入失敗。 | 每個告警都要對應到處理動作，否則只是噪音。 | 哪個指標最先告訴你出事？ |
| L2 | 在 Prometheus 怎麼算列表 API 的 p95？ | `histogram_quantile(0.95, sum by (le, route) (rate(http_request_duration_seconds_bucket{route="/api/proposals"}[5m])))`。 | 結果是在 bucket 之間線性內插的估計值。bucket 邊界要包含 `0.3`，「p95 < 300ms」的驗收才準。 | Histogram 和 Summary 差在哪？ |
| L2 | metric 的 label 不能放什麼？ | 不能放高基數的值：user id、提案 uuid、request-id、原始 URL 路徑。路徑要用路由樣板，例如 `/api/proposals/{id}`。 | 每一種 label 組合都是一條時間序列，基數爆炸會拖垮 Prometheus。 | 怎麼發現基數已經失控？ |
| L2 | EFK 怎麼收集 JSON 日誌？ | 應用只寫 stdout；Fluent Bit 以 DaemonSet 讀取 `/var/log/containers`，解析 JSON、加上 K8s metadata，送到 Elasticsearch；在 Kibana 依 `request_id` 查詢。 | 應用不要自己寫檔案或直接連 Elasticsearch。 | 多行的 stack trace 怎麼處理？ |
| L3 | request-id 怎麼一路帶到 MinIO 呼叫與稽核紀錄？ | middleware 把 request-id 放進 `context`，slog handler 從 context 取出加到每行日誌；MinIO 呼叫傳同一個 context；稽核表有 `request_id` 欄位。之後可接 OpenTelemetry 的 trace id。 | 背景排程沒有請求，要自行產生 job id。 | 怎麼從一筆稽核找到對應的日誌？ |
| L3 | 日誌記錄工號與 IP，個資法上有問題嗎？ | 這些屬於個人資料，要最小化、設保存期限、限制存取，不記提案內容與檔案。稽核紀錄另訂保存政策。 | 日誌常被複製到多個系統，比 DB 更難刪除。 | 使用者要求刪除資料時，稽核紀錄要刪嗎？ |
| L3 | 告警怎麼設才不會太吵？ | 依 SLO 的 error budget 消耗速度（burn rate）告警，並同時看長短兩個時間窗，例如 1 小時與 5 分鐘都超標才發出。不要用「CPU > 80%」這類原因型指標當主要告警。 | 原因型指標適合放在儀表板協助排查，不適合叫醒人。 | error budget 用完時要做什麼？ |
| L3 | Elasticsearch 的 mapping explosion 是什麼？ | 動態欄位（例如把整個 map 直接寫進日誌）讓 mapping 無限增長。用固定的欄位名稱、index template 定義 mapping，自由格式的欄位設為 `flattened` 或不索引。 | 同一個欄位在不同服務是不同型別（數字與字串）時會衝突，該筆日誌會被拒收。 | 日誌要保存多久？用 ILM 怎麼設定？ |

## Performance

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 為什麼分頁要設上限 100？ | 避免一次讀取大量資料拖垮 DB 與記憶體，也防止被濫用。超過上限回 422 或自動截斷，要在 API 規格中寫清楚。 | 排序欄位要加上 id 作為 tie-breaker，否則換頁時資料會重複或遺漏。 | offset 和 cursor 分頁差在哪？ |
| L2 | offset 和 cursor 分頁怎麼選？ | offset 實作簡單、可跳頁，但越後面越慢；cursor（keyset）效能穩定但不能跳頁。一萬筆的規模用 offset 可以接受。 | 回傳總筆數的 `COUNT(*)` 在篩選條件下也可能很慢。 | 要建哪些索引？ |
| L2 | 列表 API 要建哪些索引？ | 依實際篩選與排序組合建複合索引，例如 `(leader_id, created_at, id)`。用 `EXPLAIN` 確認沒有 filesort 與全表掃描。 | Member 的可見範圍要 join 成員表，那張表也要索引。 | 排序欄位是白名單嗎？ |
| L3 | 怎麼驗證 RPS ≥ 100、p95 < 300ms？ | 用 k6 之類的工具，先灌入一萬筆資料，暖機後以固定速率壓測，從結果取 p95。環境規格要接近正式環境，並記錄 DB 連線池設定。 | 要說出實際量到的數字與瓶頸在哪。 | 瓶頸在哪？你怎麼找到的？ |
| L3 | bcrypt 對容量的影響？ | cost 10 單次約數十毫秒且吃滿一個 CPU 核心。登入尖峰時要限制並發的 bcrypt 數量，避免拖慢其他 API。dummy bcrypt 也算在內。 | 攻擊者大量嘗試登入時，bcrypt 本身就是 DoS 放大器，IP throttle 要擋在 bcrypt 之前。 | 要不要把登入拆成獨立的 Deployment？ |
| L3 | Member 可見範圍的 JOIN 加上排序與 offset，p95 會在哪裡變差？ | 深 offset 要掃過並丟棄前面的列；帶篩選的 `COUNT(*)` 要掃整個符合範圍。用 `EXPLAIN` 確認走索引、沒有 filesort，必要時改用 covering index 或 cursor 分頁。 | 壓測要包含最後幾頁，不能只壓第一頁。 | 總筆數能不能改成估計值？ |
| L3 | 壓測有沒有包含認證？ | 要。每個請求都會查 session，節流後還有 `last_seen_at` 更新，這常是真正的瓶頸。壓測腳本要先登入取得 cookie。 | 只壓不需認證的路徑，數字沒有意義。 | 怎麼產生大量測試帳號？ |
| L3 | DB 連線池怎麼設？ | `Pod 數 × 每個 Pod 的連線上限` 要小於 Galera 的 `max_connections` 並保留餘裕，Pod 數以 HPA 上限計算；連線最長存活時間要小於中間 proxy 的 idle timeout。 | 連線池太大反而讓 DB 過載，p95 變差。 | 怎麼觀察連線池是否不夠用？ |
| L4 | SLO 是怎麼定的？達不到怎麼辦？ | 以使用者規模（全公司 IT 員工、截止前尖峰）推估需求。達不到時先找瓶頸，再決定加索引、加快取或加資源，必要時和需求方調整目標。 | 不要直接加 Redis，先確認瓶頸。 | 截止前一小時的尖峰怎麼準備？ |

## Architecture & Database

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 為什麼所有查詢都要參數化？ | 防止 SQL injection：參數不會被當成 SQL 語法解析。 | `ORDER BY` 欄位與方向無法參數化，要用白名單對應。 | 動態篩選條件怎麼組？ |
| L2 | 雙 id 策略的理由？ | 對內用 bigint 自增，索引小、寫入順序好；對外用 uuid，不洩漏筆數也無法被猜。 | MariaDB 10.5 沒有 `UUID` 型別（10.7 才有），要存 `BINARY(16)`。Galera 會依節點數設定 `auto_increment_increment`，id 會跳號。 | uuid 用 v4 還是 v7？ |
| L2 | 為什麼用模組化單體而不是微服務？ | 團隊與流量規模小，微服務的部署與跨服務一致性成本不划算。模組化保留邊界，未來需要時可拆。 | 要說明模組邊界怎麼守：`internal` 套件、禁止跨模組直接查別人的表。 | 跨模組要用同一個交易怎麼辦？ |
| L2 | API 規格怎麼管理？ | 規格先行：寫 OpenAPI，用 oapi-codegen 產生 server interface 與型別，前端也從同一份規格產生型別。 | 產生的程式碼要在 CI 檢查是否與規格同步。 | 錯誤回應的格式？ |
| L3 | 跨模組的交易怎麼處理？ | 由應用層開交易，把 `*sql.Tx` 透過參數或 unit of work 傳給各模組的 repository。例如 proposal、slides、audit 共用同一個交易。 | 不要把 tx 藏在 `context` 裡卻沒有明確約定，難以追蹤。 | 拆成微服務後這段怎麼辦？ |
| L3 | Galera 對 schema migration 的影響？ | DDL 預設用 TOI，會在全叢集同時執行並阻塞寫入；大表變更要用 RSU 或 pt-online-schema-change。MariaDB 的 DDL 會隱式 commit，golang-migrate 失敗時會留下 dirty 狀態。 | 每張表都要有主鍵，Galera 才能正確複製。 | migration 由誰執行？ |
| L3 | uuid 用 v4 還是 v7？ | 存成 `BINARY(16)` 並建唯一索引。v7 的前綴是時間，插入大致遞增，索引頁分裂少；v4 是隨機插入。v7 會洩漏建立時間，這個專案可以接受。 | 對外只出現 uuid，內部 bigint 跳號沒有影響。 | uuid 由應用程式產生還是 DB 產生？ |
| L3 | 提案列表要顯示 Leader 姓名，能直接 JOIN account 模組的表嗎？ | 模組化單體的取捨：讀取可透過 account 模組提供的查詢介面，或明確允許的唯讀 view；寫入一律由擁有該表的模組負責。 | 為了效能允許 JOIN 時要寫成規則，不能變成隨意互查。 | 怎麼用工具檢查模組邊界？ |
| L3 | 呼叫 MinIO 時能不能持有 DB 交易？ | 不行。外部呼叫可能很慢，持有交易會占住連線與鎖，在 Galera 上也拉長衝突窗口。先完成 MinIO 呼叫，再開短交易寫 DB。 | 下載也一樣：先 commit 稽核，再開始串流。 | 交易內呼叫外部服務還有什麼風險？ |
| L4 | 為什麼選 Galera？值得嗎？ | 需要同步複製的高可用，任一節點掛掉不丟資料。代價是寫入延遲、熱點衝突與運維複雜度。若寫入集中於單一節點，可避開多數衝突。 | 要能說出「如果重來，會不會選主從複製」。 | 叢集要幾個節點？為什麼是奇數？ |

## Data Modeling

> 以 MariaDB 10.5 為準。所有時間欄位存 UTC，由 `INSERT` 時以 `UTC_TIMESTAMP(6)` 寫入。約束一律明確命名，1062 才能依名稱對應錯誤碼（見 API Error Contract）。

```sql
CREATE TABLE accounts (
  id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  public_id      BINARY(16)   NOT NULL,
  employee_no    CHAR(6)      CHARACTER SET ascii NOT NULL,  -- 不用 INT：開頭的 0 會消失
  name           VARCHAR(50)  NOT NULL,
  password_hash  CHAR(60)     CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  is_admin       BOOLEAN      NOT NULL DEFAULT FALSE,        -- 由 Vault 同步
  failed_count   TINYINT UNSIGNED NOT NULL DEFAULT 0,
  locked_until   DATETIME(6)  NULL,                          -- NULL 代表未鎖定
  disabled_at    DATETIME(6)  NULL,
  created_at     DATETIME(6)  NOT NULL,
  CONSTRAINT uq_accounts_public_id   UNIQUE (public_id),
  CONSTRAINT uq_accounts_employee_no UNIQUE (employee_no),
  CONSTRAINT ck_accounts_employee_no CHECK (employee_no REGEXP '^[0-9]{6}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
  id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  token_hash    BINARY(32)  NOT NULL,                        -- SHA-256 原始 bytes，不存 hex 字串
  account_id    BIGINT UNSIGNED NOT NULL,
  csrf_token    BINARY(32)  NOT NULL,
  created_at    DATETIME(6) NOT NULL,
  last_seen_at  DATETIME(6) NOT NULL,
  CONSTRAINT uq_sessions_token_hash UNIQUE (token_hash),
  INDEX ix_sessions_account (account_id),                    -- 登出所有裝置、停用帳號時使用
  CONSTRAINT fk_sessions_account FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE login_ip_failures (
  ip            VARBINARY(16) NOT NULL,                      -- INET6_ATON() 轉換，IPv4 與 IPv6 皆可
  window_start  DATETIME    NOT NULL,                        -- 對齊到 15 分鐘
  fail_count    SMALLINT UNSIGNED NOT NULL,
  PRIMARY KEY (ip, window_start)
) ENGINE=InnoDB;

CREATE TABLE proposals (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  public_id   BINARY(16)   NOT NULL,
  leader_id   BIGINT UNSIGNED NOT NULL,
  title       VARCHAR(100) NOT NULL,                         -- collation 決定什麼叫「名稱重複」
  summary     TEXT         NOT NULL,
  version     INT UNSIGNED NOT NULL DEFAULT 1,
  created_at  DATETIME(6)  NOT NULL,
  updated_at  DATETIME(6)  NOT NULL,
  CONSTRAINT uq_proposals_public_id    UNIQUE (public_id),
  CONSTRAINT uq_proposals_leader_title UNIQUE (leader_id, title),
  INDEX ix_proposals_created (created_at, id),
  CONSTRAINT fk_proposals_leader FOREIGN KEY (leader_id) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE proposal_members (
  proposal_id  BIGINT UNSIGNED NOT NULL,
  account_id   BIGINT UNSIGNED NOT NULL,
  slot         TINYINT UNSIGNED NOT NULL,
  PRIMARY KEY (proposal_id, account_id),
  CONSTRAINT uq_members_slot UNIQUE (proposal_id, slot),
  CONSTRAINT ck_members_slot CHECK (slot BETWEEN 1 AND 4),
  INDEX ix_members_account (account_id, proposal_id),        -- 查「我參與的提案」
  CONSTRAINT fk_members_proposal FOREIGN KEY (proposal_id) REFERENCES proposals(id) ON DELETE CASCADE,
  CONSTRAINT fk_members_account  FOREIGN KEY (account_id)  REFERENCES accounts(id)
) ENGINE=InnoDB;

CREATE TABLE slides (                                        -- slides 模組擁有這張表
  proposal_id  BIGINT UNSIGNED PRIMARY KEY,                  -- 1:1，重傳時更新同一列
  object_key   VARCHAR(100) CHARACTER SET ascii NOT NULL,
  file_type    ENUM('pdf', 'pptx') NOT NULL,
  size_bytes   INT UNSIGNED NOT NULL,
  sha256       BINARY(32)   NOT NULL,
  uploaded_at  DATETIME(6)  NOT NULL,
  CONSTRAINT uq_slides_object_key UNIQUE (object_key),
  CONSTRAINT ck_slides_size CHECK (size_bytes <= 5242880),
  CONSTRAINT fk_slides_proposal FOREIGN KEY (proposal_id) REFERENCES proposals(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE contest_settings (                              -- 單列設定表
  id         TINYINT UNSIGNED PRIMARY KEY,
  opens_at   DATETIME(6) NOT NULL,
  closes_at  DATETIME(6) NOT NULL,
  CONSTRAINT ck_settings_singleton CHECK (id = 1),
  CONSTRAINT ck_settings_window    CHECK (opens_at < closes_at)
) ENGINE=InnoDB;

CREATE TABLE pending_file_deletions (
  id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  object_key   VARCHAR(100) CHARACTER SET ascii NOT NULL,
  not_before   DATETIME(6) NOT NULL,                         -- 最短保留時間，配合備份
  lease_until  DATETIME(6) NULL,
  attempts     SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  INDEX ix_pending_not_before (not_before)
) ENGINE=InnoDB;

CREATE TABLE audit_logs (
  id                 BIGINT UNSIGNED AUTO_INCREMENT,
  occurred_at        DATETIME(6)   NOT NULL,
  actor_id           BIGINT UNSIGNED NULL,                   -- 刻意不設外鍵
  actor_employee_no  CHAR(6)       CHARACTER SET ascii NULL, -- 快照當下的值
  action             VARCHAR(40)   NOT NULL,
  target_public_id   BINARY(16)    NULL,
  result             ENUM('success', 'denied', 'error') NOT NULL,
  reason             VARCHAR(100)  NULL,
  ip                 VARBINARY(16) NOT NULL,
  request_id         CHAR(36)      CHARACTER SET ascii NOT NULL,
  detail             JSON          NULL,
  hash               BINARY(32)    NULL,
  PRIMARY KEY (id, occurred_at),                             -- 分區鍵必須包含在每個唯一鍵中
  INDEX ix_audit_time_actor (occurred_at, actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
PARTITION BY RANGE COLUMNS (occurred_at) (
  PARTITION p2026_11 VALUES LESS THAN ('2026-12-01'),
  PARTITION p2026_12 VALUES LESS THAN ('2027-01-01'),
  PARTITION pmax     VALUES LESS THAN (MAXVALUE)
);
```

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 工號為什麼用 `CHAR(6)`，不用 `INT`？ | 工號是識別碼不是數值，不會拿來運算；`001234` 存成 `INT` 會遺失開頭的 0。固定長度用 `CHAR`，字元集用 `ascii`。 | 欄位長度都要說得出依據，不能只是習慣寫 255。 | 姓名 `VARCHAR(50)` 的長度依據是什麼？ |
| L1 | 1NF、2NF、3NF 在這個 schema 裡各對應什麼？ | 組員不存成逗號字串或 JSON 陣列，而是獨立一張表（1NF）；`proposals` 不存 `leader_name`，用 JOIN 取得（3NF）；截止狀態不存，即時推算。 | 能舉出本專案的例子，比背定義重要。 | 什麼時候會刻意反正規化？ |
| L1 | 為什麼每張表都要有主鍵？ | 除了唯一識別每一列，Galera 的複製也依賴主鍵；沒有主鍵的表在 Galera 上會出現效能與一致性問題。 | Galera 只支援 InnoDB。 | 主鍵用自增還是 uuid？ |
| L1 | 欄位什麼時候允許 `NULL`？ | 預設 `NOT NULL`，只有「沒有值」本身有意義時才允許，例如 `locked_until` 為 `NULL` 代表未鎖定。 | `UNIQUE` 允許多個 `NULL`；`= NULL` 永遠不成立，要用 `IS NULL`。 | 用 `NULL` 還是特殊值（sentinel）表示「沒有」？ |
| L2 | `DATETIME` 還是 `TIMESTAMP`？ | `TIMESTAMP` 在 10.5 有 2038 年上限，而且會依 session 時區自動轉換；`DATETIME` 不轉換，以「一律存 UTC」作為約定。 | 精度要用 `(6)`，否則同一秒內的事件無法排序。 | 截止時間的比較用 `<` 還是 `<=`？ |
| L2 | MariaDB 的 `utf8` 和 `utf8mb4` 差在哪？ | MariaDB 的 `utf8` 是 `utf8mb3`，存不了 emoji 與部分罕用字，一律用 `utf8mb4`。 | collation 選 `utf8mb4_unicode_ci` 或 `utf8mb4_general_ci` 會影響標題唯一性的判斷；10.5 沒有 `uca1400`（10.10 才有）。 | 標題用 `_ci` 會讓哪些標題被視為重複？ |
| L2 | token hash 為什麼用 `BINARY(32)`，不存 64 字元的 hex？ | 空間減半、索引更小；二進位比較沒有 collation 的問題。 | hex 存在 `_ci` 欄位時，大小寫不同的字串會被視為相同。 | `BINARY(16)` 的 uuid 除錯時怎麼閱讀？ |
| L2 | `ENUM`、`VARCHAR` 加 `CHECK`、lookup table 怎麼選？ | `ENUM` 省空間，但新增值要 `ALTER TABLE`，在 Galera 上以 TOI 執行，會阻塞整個叢集的寫入。值固定的（`file_type`）用 `ENUM`，會持續增加的（`action`）用 `VARCHAR`。 | `ENUM` 依宣告順序排序，不是依字母。 | 稽核的 `action` 為什麼不用 `ENUM`？ |
| L2 | `slides` 為什麼和 `proposals` 分表？ | 模組邊界：slides 模組擁有自己的表。兩者是 1:1，以 `proposal_id` 作為主鍵。 | 分表後列表 API 需要 JOIN，要評估效能。 | 上傳簡報要不要遞增提案的 version？ |
| L2 | 外鍵要設哪些？`CASCADE` 放在哪裡？ | 組員與簡報隨提案以 `CASCADE` 刪除；`audit_logs` 刻意不設外鍵。 | `CASCADE` 刪除不會觸發 trigger，也不會經過應用層，稽核要由應用層自己寫。 | 外鍵在 Galera 上有什麼代價？ |
| L3 | 稽核表為什麼刻意不正規化？ | 稽核記的是歷史事實：使用者改名或被停用後，紀錄必須保留當時的值，所以存快照，不靠 JOIN 取得。 | 這是「刻意反正規化」的典型例子，要能和 3NF 的題目對照著答。 | 個資刪除的要求和稽核保存怎麼取捨？ |
| L3 | soft delete 和 `UNIQUE (leader_id, title)` 會衝突嗎？ | 會。軟刪除的列仍然佔住名稱；若把 `deleted_at` 加進唯一鍵，未刪除的列都是 `NULL`，彼此不相等，唯一性就失效。可以用生成欄位解決，或改用硬刪除。 | 規格說刪除提案就刪除檔案，硬刪除加上稽核最單純。 | 刪除之後需要能復原嗎？ |
| L3 | 稽核表按月分區，主鍵為什麼變成 `(id, occurred_at)`？ | MariaDB 要求分區鍵包含在每一個唯一鍵中。 | 分區表不能有外鍵。 | 舊分區怎麼歸檔與刪除？ |
| L3 | 複合索引的欄位順序怎麼排？ | 等值條件在前，範圍條件其次，排序欄位最後；遵守最左前綴；必要時做 covering index。 | 用 `EXPLAIN` 驗證，不要靠猜。 | 列表 API 要建哪些索引？ |
| L3 | MariaDB 的 `JSON` 和 MySQL 的有什麼不同？ | MariaDB 的 `JSON` 是 `LONGTEXT` 的別名，加上 `JSON_VALID` 檢查，不是二進位格式。要查詢其中的值，用虛擬生成欄位再建索引。 | 需要查詢的欄位不要塞進 `detail`。 | `detail` 要放什麼、不放什麼？ |
| L3 | `VARCHAR(255)` 在 `utf8mb4` 下能建索引嗎？ | 可以，最多 1020 bytes，在 InnoDB DYNAMIC row format 的 3072 bytes 上限內。複合索引的長度要加總計算。 | | 標題很長又要唯一時怎麼辦？ |
| L3 | 預設隔離等級是 REPEATABLE READ，在 Galera 上有什麼差別？ | 單一節點內有 gap lock；跨節點只在 commit 時做 certification，`FOR UPDATE` 的鎖不跨節點。 | 這也是組員上限選擇 slot 方案的原因。 | 改成 READ COMMITTED 會怎樣？ |
| L4 | 要不要加 `member_count` 這類反正規化欄位？ | 一萬筆的規模用 JOIN 或 `COUNT` 即可。反正規化之後每次寫入都要同步維護，在 Galera 上還會變成熱點列。 | 先量測再決定。 | 到什麼量級你會改變決定？ |
| L4 | 規格之後要支援「一份提案保存多個簡報版本」，schema 怎麼演進？ | `slides` 從 1:1 改成 1:N，提案加一個 `current_slide_id` 指向目前的版本；用 expand / contract 分次上線。 | 會影響 `pending_file_deletions` 的保留策略。 | 舊版本的檔案要保留多久？ |

## Security

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 前後端同源，為什麼不需要 CORS？ | CORS 是放寬同源政策的機制；同源請求本來就不受限制。 | 同源不代表不需要防 CSRF。 | CSRF 怎麼防？ |
| L2 | CSRF 怎麼防？ | Session cookie 設 `SameSite`，並對所有寫入請求要求 CSRF token（或自訂 header），在認證之後、授權之前檢查。 | `SameSite=Lax` 仍允許頂層 GET 導航，所以 GET 不能有副作用。 | token 怎麼發給 SPA？ |
| L2 | CSP 怎麼設？ | `default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'`，由 Nginx 加上。另外設 HSTS、`X-Content-Type-Options: nosniff`、`Referrer-Policy`。 | 靜態 SPA 難以逐請求產生 nonce，內嵌 script 要改成外部檔案或用 hash。 | 第三方套件需要 inline style 怎麼辦？ |
| L3 | 上傳的 PDF 會帶來什麼風險？ | PDF 可內嵌 JavaScript；若同源直接開啟，可能在網站的 origin 下執行。下載時一律設 `Content-Disposition: attachment` 與 `nosniff`。 | 如果之後要做線上預覽，要放在獨立網域或 sandbox iframe。 | 要不要做內容消毒？ |
| L3 | SPA 的 CSRF token 從哪裡來？要不要綁定 session？ | 同步權杖：token 存在 session 列，登入後由 API 回傳，寫入請求帶 `X-CSRF-Token`。也可用 double-submit cookie。只要求自訂 header 也有效，因為跨站請求無法在沒有 CORS 預檢通過的情況下帶自訂 header。 | 自訂 header 的防線前提是不開 CORS；之後若開放 CORS 要重新評估。 | 登入請求本身要防 CSRF 嗎（login CSRF）？ |
| L3 | API 與下載回應也要設 CSP 嗎？ | 要。對 API 與下載回應加 `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`，萬一檔案被當成頁面渲染也無法執行。 | SPA 的 CSP 由 Nginx 設，API 的由後端 middleware 設，兩者內容不同。 | 怎麼驗證每個回應都帶到 header？ |
| L4 | 這個系統的威脅模型是什麼？ | 內部使用者為主：關注越權查看他人提案、截止後竄改、Admin 濫權與資料外洩。外部攻擊面較小，防護投資依此分配。 | 「內部系統」不等於可以省略防護，內部人員是主要威脅來源。 | 哪一項你認為最危險？ |
| L1 | 憑證信任鏈怎麼驗證？ | root CA → intermediate CA → leaf 憑證。client 逐層檢查簽章、有效期、SAN 是否符合網域，以及是否被撤銷，最後要能連到信任庫裡的 root。 | 伺服器要送出完整的鏈（leaf 加 intermediate），否則部分 client 會驗證失敗。 | 為什麼 root 不放在伺服器送出的鏈裡？ |
| L1 | 為什麼用 intermediate CA，而不是直接用 root 簽發？ | root 私鑰離線保存，只用來簽 intermediate。intermediate 洩漏時只撤銷它，不必重發 root、也不必更新所有 client 的信任庫。 | root 一旦洩漏，所有 client 的信任庫都要更新，成本極高。 | root 私鑰放在哪裡？ |
| L2 | 憑證過期會發生什麼？怎麼避免？ | 瀏覽器擋連線，API client 報錯，服務等同中斷。要自動更新（cert-manager、ACME），並對剩 30 天、14 天的憑證告警。 | 自動更新失敗要有告警，不能只靠續期成功。 | 怎麼找出叢集中所有即將到期的憑證？ |
| L2 | CN 和 SAN 差在哪？ | 現在的瀏覽器只看 SAN，CN 已被忽略。憑證要把所有網域與 wildcard 都列在 SAN。 | wildcard（`*.example.com`）只涵蓋一層子網域。 | wildcard 憑證有什麼風險？ |
| L2 | 憑證撤銷怎麼做？CRL 和 OCSP 差在哪？ | CRL 是整份撤銷清單，要下載；OCSP 是逐張即時查詢。OCSP stapling 由伺服器附上簽過的回應，減少延遲與隱私問題。 | 很多 client 對撤銷檢查是 soft-fail，所以短效憑證更實際。 | 為什麼現在趨勢是縮短憑證效期？ |
| L3 | 公開 CA 和內部 CA 怎麼選？ | 對外服務用 Let's Encrypt 或商用 CA，瀏覽器已信任。內部服務與 mTLS 用內部 CA，要把 root 發到所有 client 的信任庫。 | 內部 CA 要承擔簽發、輪替、撤銷與私鑰保管的維運責任。 | 信任庫怎麼發到所有 Pod？ |
| L3 | mTLS 怎麼運作？什麼情況下要用？ | 雙方都出示憑證並互相驗證。用在服務間零信任或合規要求端對端加密的場景。 | 代價是每個服務都要有憑證的發放與輪替；service mesh 可自動處理。 | 這個系統需要 mTLS 嗎？ |

## Attack Scenarios

> 從攻擊者的角度出題：給出一個攻擊，問怎麼防、怎麼測。

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L2 | 「uuid 猜不到，所以不需要再檢查授權」對嗎？ | 不對（IDOR / BOLA）。uuid 只降低猜中的機率，仍會從日誌、分享的連結、瀏覽紀錄外洩。每個請求都要檢查授權。 | uuid v7 的前綴是時間，比 v4 更容易縮小範圍。 | 怎麼測 IDOR？ |
| L2 | 攻擊者在整筆替換的 body 加上 `leader_id`、`version`、`is_admin`？ | Mass assignment。用明確的 request 型別，只對應允許的欄位；OpenAPI 設 `additionalProperties: false`，並在驗證時拒絕未知欄位。 | 把 request body 直接綁定到 ORM entity，或用展開運算子（`...body`）寫入，最容易出事。 | `version` 為什麼只能從 `If-Match` 來？ |
| L2 | 提案標題填入 `<img src=x onerror=alert(1)>`？ | Stored XSS。React 預設會跳脫輸出；危險點在 `dangerouslySetInnerHTML`、`href` 帶 `javascript:`、Admin 後台與 Excel 匯出。CSP 是第二道防線。 | 只在輸入端過濾不夠，要在輸出端依情境跳脫。 | 之後支援 Markdown 描述呢？ |
| L3 | XSS 讀不到 `HttpOnly` cookie，所以 session 是安全的？ | 不對。攻擊腳本可以直接在頁面內帶著 cookie 發出請求（session riding），也能讀到頁面上的 CSRF token。 | `HttpOnly` 只防止 token 被帶走；真正的防線是不讓腳本執行，也就是 CSP 與輸出跳脫。 | 怎麼驗證 CSP 真的有擋住？ |
| L3 | 上傳的 PPTX 裡，`[Content_Types].xml` 帶了外部實體宣告？ | XXE：XML parser 若處理外部實體，可能讀取伺服器檔案或發出內部請求。Go 的 `encoding/xml` 不處理外部實體；Java 要關閉 DTD（`disallow-doctype-decl`）。 | 驗證檔案時只讀必要的 entry，不要把整個 zip 解開到磁碟。 | zip slip 是什麼？ |
| L3 | 登入頁的 `?next=https://evil.example`？ | Open redirect。只接受站內的相對路徑：以 `/` 開頭，但不是 `//` 或 `/\`。 | 釣魚連結用的是公司的網域，使用者更容易相信。 | 為什麼內部系統也要在意？ |
| L3 | 同時送出 5 個加入組員的請求，想讓團隊超過 4 人？ | Race condition。`UNIQUE (proposal_id, slot)` 讓多出的請求失敗，回 409。 | 「先查人數再寫入」在並發下一定會被突破。 | 怎麼寫測試重現？ |
| L3 | 在姓名或 `X-Request-ID` 放入換行與偽造的日誌內容？ | Log injection。JSON handler 會跳脫換行；純文字日誌才危險。外部傳入的值要驗證格式與長度。 | 偽造的日誌可能誤導事件調查。 | 日誌要怎麼防竄改？ |
| L3 | 前端的 Nginx Pod 被入侵了，攻擊者下一步能做什麼？ | 沒有 NetworkPolicy 時，可以直接連 MariaDB 與 MinIO；自動掛載的 ServiceAccount token 可以呼叫 K8s API。要設 default deny 的 NetworkPolicy、`automountServiceAccountToken: false`、最小權限的 RBAC。 | 容器以 root 執行或可寫入根檔案系統，會讓攻擊者更容易留下後門。 | 怎麼偵測異常的連線？ |
| L3 | Admin 帳號被盜？ | 影響最大：可以查看、修改所有提案並匯出資料。Admin 操作可要求較短的 session、重新驗證密碼或 MFA；稽核異常的大量下載或匯出並發出告警。 | Admin 名單由 Vault 同步，Vault 的存取權限也是攻擊面。 | 怎麼定義「異常」？ |
| L4 | 用 STRIDE 對這個系統做威脅建模？ | Spoofing：冒用他人工號註冊。Tampering：截止後竄改、修改稽核紀錄。Repudiation：沒有稽核就無法追究。Information disclosure：越權查看、帳號枚舉。DoS：bcrypt 與上傳耗盡資源。Elevation of privilege：Admin 同步錯誤。再依影響與可能性排序。 | 要能說出最優先處理哪一項，以及理由。 | 威脅模型多久要重新檢視？ |

## DevOps

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 為什麼用 distroless？ | 映像只有執行所需的檔案，沒有 shell 和套件管理器，攻擊面小、掃描出的 CVE 少、體積小。 | Go 用 `CGO_ENABLED=0` 編成靜態檔，可用 `distroless/static`；以 nonroot 使用者執行。 | 沒有 shell 怎麼除錯？ |
| L1 | Structured logging 怎麼做？ | 用 `log/slog` 的 JSON handler；middleware 產生或沿用 request-id，放進 context，每行日誌都帶出來，也寫進稽核。 | 外部傳入的 `X-Request-ID` 要驗證格式與長度，避免日誌注入。 | 日誌怎麼蒐集？ |
| L2 | CI 的各項安全掃描分別負責什麼？ | Whitesource（Mend）掃相依套件的 CVE 與授權（SCA）；Semgrep 掃原始碼（SAST）；Base Image Scan 掃映像的系統套件；Secret Detect 抓提交進版控的機密。 | 要說明哪些等級會擋住 pipeline，誤判如何處理（例外清單要有期限與核准）。 | 掃到高風險 CVE 但沒有修補版本怎麼辦？ |
| L2 | 機密怎麼注入 Pod？ | 由 Vault 提供，透過 Vault Agent Injector、CSI driver 或 External Secrets Operator 注入。Admin 名單也從 Vault 同步。 | 不要把機密寫進 Helm values 或映像。 | 機密輪替時 Pod 怎麼拿到新值？ |
| L3 | Azure Release 和 ArgoCD 怎麼分工？ | CI 建置並推送映像；Release 負責晉升流程（環境、核准），做法是修改 GitOps repo 裡的 Helm values（映像 tag）。ArgoCD 偵測 Git 變更並同步到叢集。 | Release 不直接對叢集 `kubectl apply`，叢集狀態只以 Git 為準。 | 怎麼回滾？ |
| L3 | migration 由誰執行？滾動更新時怎麼確保相容？ | 用 Helm pre-upgrade hook 或 ArgoCD 的 PreSync Job 執行，只跑一次。schema 變更採 expand / contract：先加欄位，新舊版本都能運作，下一版才移除舊欄位。 | 不要讓每個 Pod 啟動時各自跑 migration。 | migration 失敗卡在 dirty 狀態怎麼救？ |
| L3 | migration 在 Galera 上執行到一半失敗？ | MariaDB 的 DDL 會隱式 commit，無法 rollback，golang-migrate 標記為 dirty。手動確認 schema 的實際狀態後執行 `migrate force <version>`，修正後往前推（fix forward）。映像回滾時，舊版程式要能在已擴充的 schema 上運作。 | 這正是要採用 expand / contract 的原因。 | 怎麼在上線前發現 migration 問題？ |
| L3 | 除了掃描，供應鏈還能做什麼？ | 產生 SBOM、用 cosign 簽署映像、叢集用 admission policy 只允許已簽署的映像、base image 以 digest 固定版本。 | 只用 tag 固定不夠，tag 可以被覆寫。 | 誰管理簽章金鑰？ |
| L4 | Galera 和 MinIO 要跑在 K8s 裡嗎？ | 有狀態服務在 K8s 需要 StatefulSet、PV、備份與升級流程，維運成本高。若公司已有託管的 DB 和物件儲存，優先使用。 | 要說出備份與還原演練怎麼做。 | RPO 和 RTO 是多少？ |
| L1 | cert-manager 在做什麼？ | 用 CRD（`Issuer`、`ClusterIssuer`、`Certificate`）宣告需要的憑證，自動向 CA 申請、更新，並存進 `Secret`。 | 憑證與私鑰都在 `Secret` 裡，要限制讀取權限。 | 憑證什麼時候更新？ |
| L2 | `Issuer` 和 `ClusterIssuer` 差在哪？ | `Issuer` 只限同一個 namespace；`ClusterIssuer` 全叢集共用。 | 共用的 `ClusterIssuer` 要搭配 RBAC 與 policy，避免任何 namespace 都能簽任意網域。 | 怎麼限制哪些網域可以簽發？ |
| L2 | ACME 的 HTTP-01 和 DNS-01 怎麼選？ | HTTP-01 設定簡單，但不能簽 wildcard，且要對外開放 80 port。DNS-01 能簽 wildcard，也適合內網服務，但要有 DNS API 權限。 | 內部服務無法對外開放時，只能用 DNS-01 或內部 CA。 | DNS API 的憑證怎麼保管？ |
| L2 | Ingress 怎麼自動取得憑證？ | 在 Ingress 加 annotation `cert-manager.io/cluster-issuer`，並在 `tls` 區塊指定 `hosts` 與 `secretName`，cert-manager 會自動建立 `Certificate`。 | Ingress 沒有 `tls` 時，`Secure` cookie 送不出去。 | 申請失敗時怎麼排查？ |
| L3 | 憑證更新後，Pod 怎麼拿到新憑證？ | ingress-nginx 會監看 `Secret` 並自動載入。自己的服務若以 volume 掛載，檔案會更新，但程式要重新讀取或重啟，可用 Reloader 或 sidecar。 | 掛 `subPath` 的 volume 不會自動更新。 | 怎麼驗證新憑證確實生效？ |
| L3 | 內部 CA 的私鑰放哪裡？ | 放 Vault PKI 或 HSM，不放 Git，也不放 Helm values。cert-manager 用 Vault `Issuer` 向 Vault 簽發，私鑰不離開 Vault。 | root 離線，只有 intermediate 放在線上簽發。 | Vault 本身掛掉時，憑證更新會怎樣？ |
| L3 | 自簽 CA 的 root 怎麼發給所有 client？ | 用 trust-manager 產生並分發 CA bundle，或打包進基底映像。 | 映像打包的方式在 CA 輪替時要重建所有映像。 | CA 要輪替時怎麼平滑切換？ |
| L4 | 內部 CA 私鑰洩漏時的處理流程？ | 撤銷並停用該 CA、建立新 CA、重發所有憑證、重發信任庫，並調查洩漏範圍。 | 要有演練過的 runbook，否則信任庫重發會耗時很久。 | 怎麼縮短這段時間？ |

## Kubernetes & Helm

### 找錯題

下面的 manifest 至少有十個問題，請逐一指出：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: esg
spec:
  ingressClassName: nginx
  rules:
    - host: esg.example.internal
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service: { name: esg-api, port: { number: 8080 } }
          - path: /
            pathType: Prefix
            backend:
              service: { name: esg-web, port: { number: 80 } }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: esg-api
spec:
  replicas: 1
  selector:
    matchLabels: { app: esg-api }
  template:
    metadata:
      labels: { app: esg-api }
    spec:
      containers:
        - name: api
          image: registry.example.internal/esg-api:latest
          env:
            - name: DB_PASSWORD
              value: "P@ssw0rd"
          ports:
            - containerPort: 8080
          readinessProbe:
            httpGet: { path: /healthz/storage, port: 8080 }
          livenessProbe:
            httpGet: { path: /healthz/db, port: 8080 }
```

| 問題 | 影響 | 修正 |
| --- | --- | --- |
| Ingress 沒有設 `proxy-body-size` | ingress-nginx 預設上限 1m，5 MiB 的上傳被擋下，回 413 | `nginx.ingress.kubernetes.io/proxy-body-size: "6m"`，要略大於 5 MiB 以容納 multipart 的其他欄位 |
| Ingress 沒有 `tls` | session cookie 設了 `Secure` 就送不出去；不設則明文傳輸 | 加上 `tls` 與憑證 |
| 映像用 `:latest` | 無法得知實際部署的版本，也無法回滾 | 用版本 tag，最好以 digest 固定 |
| 密碼寫在 `env.value` | 機密進了 Git 與 Helm values | 由 Vault 注入 |
| `replicas: 1`，沒有 PDB | 節點維護或滾動更新時服務中斷 | 至少 2 個 replica，加上 `PodDisruptionBudget` |
| readiness 檢查 MinIO | MinIO 一掛，所有 Pod 被移出 Service，連列表 API 都不能用 | readiness 只檢查 DB |
| liveness 檢查 DB | DB 一掛，所有 Pod 被重啟，造成重啟風暴 | liveness 只檢查程式本身是否還在運作 |
| 沒有 `resources` | 無法正確排程，也可能吃光節點資源 | 依實測設定 `requests`，記憶體設 `limits` |
| 沒有 `securityContext` | 以 root 執行、可寫入根檔案系統 | `runAsNonRoot`、`readOnlyRootFilesystem`、`allowPrivilegeEscalation: false`、`capabilities.drop: [ALL]` |
| 根檔案系統唯讀後沒有 `/tmp` | multipart 寫暫存檔失敗 | 掛一個 `emptyDir` 到 `/tmp` |
| 沒有 `automountServiceAccountToken: false` | Pod 被入侵時可以呼叫 K8s API | 不需要時關閉 |
| 沒有 graceful shutdown 設定 | 滾動更新時，進行中的上傳被中斷 | `preStop` 加上適當的 `terminationGracePeriodSeconds` |

### 題目

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | Helm chart 的結構？ | `Chart.yaml` 描述 chart；`values.yaml` 是預設值；`templates/` 放樣板；`_helpers.tpl` 放共用的命名與 label。各環境用 `values-<env>.yaml` 覆寫。 | 預設值要是安全的設定，環境只覆寫差異。 | 部署前怎麼看渲染出來的結果？ |
| L1 | 部署前怎麼確認 manifest 正確？ | `helm template` 渲染、`helm lint` 檢查；`helm diff` 比對與叢集現況的差異。ArgoCD 的 UI 也會顯示 diff。 | 可在 CI 用 kubeconform 驗證 schema，再用 policy 工具檢查安全設定。 | values 怎麼驗證型別？ |
| L2 | ConfigMap 改了，Pod 卻沒有重啟？ | Deployment 的 Pod template 沒有變，就不會滾動更新。在 Pod annotation 加上 `checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . \| sha256sum }}`。 | 以檔案方式掛載的 ConfigMap 會自動更新內容，但程式不一定會重新讀取。 | Vault 注入的機密輪替時呢？ |
| L2 | `requests` 和 `limits` 怎麼設？ | `requests` 依實測用量設定，影響排程與 HPA；記憶體超過 `limits` 會被 OOMKilled。CPU `limits` 會造成 throttling，拉高 p95，很多團隊只設 CPU `requests`。 | bcrypt 是 CPU 密集，登入尖峰時最容易被 throttle。 | 怎麼從 metrics 看出 CPU throttling？ |
| L2 | 為什麼需要 PDB？ | 節點維護（`kubectl drain`）時，PDB 保證至少有幾個 Pod 仍在運作。 | `minAvailable` 等於 replica 數時，drain 會永遠卡住。 | HPA 縮到 1 個 Pod 時呢？ |
| L3 | 收到 SIGTERM 時，正在上傳或下載的請求怎麼辦？ | `preStop` 先等待幾秒，讓 Endpoints 移除這個 Pod；再呼叫框架的 graceful shutdown（Go 是 `server.Shutdown(ctx)`），等待進行中的請求。等待時間要小於 `terminationGracePeriodSeconds`。被中斷的上傳留下孤兒物件，由排程清除。 | distroless 沒有 `sleep` 指令，不能用 `exec` 執行；K8s 1.30 起可以用內建的 `sleep` 動作。背景排程也要接收 context 取消。 | 滾動更新時怎麼確認沒有請求失敗？ |
| L3 | NetworkPolicy 怎麼設？ | 預設全部拒絕；只允許 Ingress → web、Ingress → api、api → MariaDB、MinIO、Vault。 | 要記得允許 DNS（kube-dns 的 53 port），否則所有連線都會失敗。 | CNI 不支援 NetworkPolicy 時怎麼辦？ |
| L3 | HPA 依什麼指標擴展？ | CPU 適合 bcrypt 這類運算；串流 I/O 不吃 CPU，CPU 指標不準，可改用請求數或並發數。 | HPA 上限 × 每個 Pod 的連線池大小，要小於 DB 的 `max_connections`。 | 截止前的尖峰要不要預先擴展？ |
| L3 | values 怎麼設計，才不會在覆寫時出錯？ | 用 `values.schema.json` 驗證型別與必填欄位；機密只放 Vault 的路徑，不放實際的值。 | `helm upgrade --reuse-values` 會沿用舊的值，容易漏掉新增的欄位。 | chart 本身怎麼做版本管理？ |

## Nginx & Web Delivery

> 本專案有兩層 Nginx：叢集入口的 ingress-nginx，以及提供 SPA 靜態檔的前端 Pod。兩層要分清楚各自負責什麼。

### SPA Server

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | SPA 為什麼需要 `try_files $uri /index.html`？不設會怎樣？ | SPA 的路由由前端處理，伺服器上沒有 `/proposals/123` 這個檔案。不設的話，重新整理或直接開子路徑會得到 404。`try_files` 先找實際檔案，找不到就回 `index.html`。 | `/api/` 要有獨立的 `location`，否則打錯的 API 路徑會拿到 200 的 HTML。 | API 路徑打錯時，怎麼避免回 `index.html`？ |
| L2 | 靜態資源和 `/api` 的 `location` 優先順序？ | 依序：`=` 精確匹配、`^~` 前綴匹配、regex（依設定檔順序）、最長前綴。要讓 `/api/` 不被副檔名的 regex（例如 `\.js$`）攔走，可用 `location ^~ /api/`。 | 同一份設定裡混用 regex 與前綴，最容易發生預期外的匹配。 | 怎麼測試某個 URL 會進哪個 `location`？ |
| L2 | 兩層 Nginx（ingress-nginx 與前端 Pod）各負責什麼？ | ingress-nginx：TLS 終止、依路徑分流到 web 或 api、`proxy-body-size`、限流。前端 Pod：提供靜態檔、`try_files`、快取與安全 header。API 的 header 由後端 middleware 設定。 | 同一個 header 在兩層都設，會出現重複值。 | 為什麼不讓 ingress-nginx 直接提供靜態檔？ |
| L3 | 前端 Pod 用非 root 與唯讀根檔案系統，Nginx 要掛哪些路徑？ | 使用 `nginx-unprivileged`（監聽 8080）；`/var/cache/nginx`、`/tmp`（pid 與暫存檔）掛 `emptyDir`。 | 一般的 `nginx` 映像要綁 80 port 與寫入 `/var/run`，在 `runAsNonRoot` 下會啟動失敗。 | 怎麼確認沒有其他路徑需要寫入？ |
| L3 | 新版部署後，使用者還拿著舊的 `index.html`，指向已刪除的 hash 檔？ | 舊 `index.html` 引用的 JS 在新 Pod 上不存在，回 404 後白畫面。可保留前一版的資源一段時間，或在前端偵測載入失敗後重新整理。 | 滾動更新期間新舊 Pod 並存，請求可能被分到沒有該檔的 Pod。 | 前端怎麼得知有新版本？ |

### Reverse Proxy

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | `proxy_pass http://api;` 加不加結尾 `/`，差在哪？ | 不帶 URI 時，原始路徑原樣轉發；帶了 URI（含單獨的 `/`）時，匹配到的 `location` 前綴會被取代。例如 `location /api/` 搭配 `proxy_pass http://api/;` 會把 `/api/x` 轉成 `/x`。 | 結尾 `/` 差一個字元，後端收到的路徑就不同，是最常見的 404 原因。 | 變數形式的 `proxy_pass` 又怎麼處理？ |
| L2 | 後端怎麼拿到真實的 client IP 與 scheme？ | Nginx 設 `X-Forwarded-For`、`X-Forwarded-Proto`（也可用 `X-Real-IP`）；後端只信任來自已知 proxy 的值。標準化版本是 `Forwarded` header（RFC 7239）。 | IP throttle 若讀到的是 Ingress 的 IP，全公司會共用同一個計數。 | 多層 proxy 時要取 `X-Forwarded-For` 的哪一個值？ |
| L2 | `set_real_ip_from` 與 `real_ip_recursive` 做什麼？ | 指定哪些上游 proxy 可信，Nginx 才會把 `X-Forwarded-For` 的值還原成 `$remote_addr`；`real_ip_recursive on` 會由右往左跳過可信位址，取第一個不可信的。 | 不設時，`$remote_addr` 是前一層 proxy 的 IP，限流與稽核的來源 IP 都會錯。 | client 自己偽造 `X-Forwarded-For` 會怎樣？ |
| L3 | client 偽造 `X-Forwarded-For`，要追加還是覆寫？ | 邊界（最外層）的 proxy 應覆寫，丟棄 client 送來的值；內部各層再用 `$proxy_add_x_forwarded_for` 追加。只信任自己 proxy 寫入的那一段。 | 取最左邊的值最容易被偽造，取最右邊且可信的才安全。 | 稽核紀錄要記哪一個 IP？ |
| L3 | 上傳 5 MiB 時，`proxy_request_buffering` 開或關的差異？ | 預設開：Nginx 收完整個請求才轉給後端，後端不受慢速客戶端拖累，但會佔暫存空間；關閉則邊收邊轉，後端要自己承擔慢速連線。 | 開啟時，超過 `client_body_buffer_size` 會寫暫存檔，唯讀根檔案系統要有可寫路徑。 | 慢速上傳（slowloris）怎麼防？ |
| L3 | 下載大檔要不要 `proxy_buffering off`？ | 預設開：Nginx 先緩衝後端回應，再慢慢送給 client，後端可以早點釋放。大檔下載若緩衝，會寫到磁碟暫存；關閉則 Nginx 直接串流，後端的 `io.Copy` 與 client 速度綁在一起。 | 要在「後端被慢速 client 占住」和「Nginx 磁碟暫存」之間取捨。 | 怎麼限制單一 client 的下載頻寬？ |
| L3 | 502、503、504 各代表哪一段出問題？ | 502：連得上後端但回應無效，或連線被重設；503：沒有可用的後端，或被限流；504：後端沒在 timeout 內回應。看 error log 的訊息（`connect() failed`、`upstream timed out`、`upstream prematurely closed`）才能分辨。 | 這三種都是 Nginx 自己產生的，不是 problem+json，前端要能處理。 | 這三種錯誤該不該重試？ |
| L3 | `proxy_read_timeout` 預設 60 秒，與其他層的 timeout 怎麼配合？ | 外層要大於內層：Nginx 的 proxy timeout > 後端 server timeout > MinIO 呼叫 > DB 查詢。上傳與下載要另設較長的值。 | `proxy_read_timeout` 是「兩次讀取之間」的間隔，不是整個請求的總時間。 | 怎麼從日誌看出是哪一層逾時？ |
| L3 | 後端 Pod 滾動更新後，Nginx 還連到舊 IP？ | `proxy_pass` 的主機名稱只在 Nginx 啟動或 reload 時解析一次。要連 K8s Service（ClusterIP 固定）；若必須用 Pod DNS，要加 `resolver` 並以變數寫 `proxy_pass` 才會重新解析。 | ingress-nginx 直接管理 Endpoints，沒有這個問題；自己寫的 Nginx 才會遇到。 | 為什麼用變數就會重新解析？ |

### Load Balancer

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | round-robin、`least_conn`、`ip_hash` 各適用什麼情境？ | round-robin 適合請求成本相近；`least_conn` 適合請求時間差異大（例如有大檔上傳下載）；`ip_hash` 讓同一 IP 固定到同一後端。 | `ip_hash` 在公司 NAT 後大量使用者共用一個 IP，會造成負載不均。 | 為什麼本專案不需要 `ip_hash`？ |
| L2 | session 存在 DB，需要 sticky session 嗎？ | 不需要。任一 Pod 都能依 cookie 的 token 查 DB 驗證，後端是無狀態的，LB 可以自由分配。 | session 若存在記憶體，才需要 sticky，同時也失去滾動更新與擴縮的彈性。 | 為什麼不用 Redis 存 session？ |
| L2 | 開源版 Nginx 的健康檢查怎麼運作？ | 被動式：實際請求失敗達 `max_fails` 次，就在 `fail_timeout` 內暫時移出。沒有主動探測，主動檢查是 Nginx Plus 的功能。在 K8s 由 readiness probe 決定 Pod 是否在 Endpoints 內。 | 被動式必須讓真實使用者的請求先失敗一次。 | 容器環境下，健康檢查該放在哪一層？ |
| L3 | 某個後端 Pod 很慢但沒掛，LB 會怎麼做？ | 不會移除，繼續分配請求，p95 被拖慢。用 `least_conn` 降低影響，並設定合理的 `proxy_read_timeout` 與重試策略；根本上要靠 readiness probe 與 metrics 找出問題 Pod。 | 慢比掛更難處理，健康檢查只回答「活著嗎」，不回答「快嗎」。 | 怎麼從 metrics 找出慢的那一個 Pod？ |
| L3 | `proxy_next_upstream` 對 POST 重試，會不會重複建立提案？ | 預設不對非冪等的請求（POST）重試，由 `non_idempotent` 選項控制。若打開，請求可能已被第一個後端處理過，造成重複建立。 | 這個專案有 `UNIQUE (leader_id, title)` 可擋下，但不是所有操作都有這種保護。 | 要怎麼讓 POST 可以安全重試？（`Idempotency-Key`） |
| L3 | 沒設 upstream `keepalive`，高 QPS 下會怎樣？ | 每個請求都新建 TCP 連線，造成大量 `TIME_WAIT`、可能耗盡暫存 port 且延遲變高。要設 `keepalive`、`proxy_http_version 1.1`，並清空 `Connection` header。 | HTTP/1.0 預設不維持連線，Nginx 對 upstream 預設就是 1.0。 | `keepalive` 的數字代表什麼？ |
| L3 | L4 與 L7 負載平衡差在哪？ | L4 依 IP 與 port 轉送 TCP 連線，看不到 HTTP 內容；L7 能依路徑、header 路由、做 TLS 終止與重試。kube-proxy 與雲端 NLB 偏 L4，Ingress 與 Nginx 是 L7。 | gRPC 與長連線在 L4 下，連線一旦建立就固定在同一個後端，負載不均。 | 為什麼 HTTP/2 會讓 L4 的平衡失效？ |
| L4 | 前面已有 ingress-nginx，前端 Pod 再放一層 Nginx 值得嗎？ | 值得：職責分離，前端 Pod 只管靜態檔與前端安全 header，可獨立部署與回滾；代價是多一層與多一個維運對象。規模小時也可把靜態檔打包進後端。 | 要說出什麼條件下會合併。 | 靜態檔改放物件儲存加 CDN 呢？ |

### Cache

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | `index.html` 與帶 hash 的資源，`Cache-Control` 各設什麼？ | `index.html`：`no-cache`（每次都向伺服器驗證，用 `ETag` 回 304）；帶 hash 的 JS/CSS：`public, max-age=31536000, immutable`。 | `no-cache` 不是「不快取」，是「使用前要驗證」；`no-store` 才是完全不存。 | `no-cache` 與 `no-store` 差在哪？ |
| L2 | `no-cache`、`no-store`、`must-revalidate` 的差異？ | `no-cache`：可存，但每次使用前必須驗證；`no-store`：不得存放；`must-revalidate`：過期後必須驗證，不可使用過期內容。 | 含授權資料的 API 與下載回應要用 `no-store`。 | `private` 與 `public` 呢？ |
| L2 | 靜態檔的 `ETag` 與 304 怎麼運作？多個 Pod 會不一致嗎？ | 瀏覽器帶 `If-None-Match`，相同回 304 不傳 body。Nginx 的 ETag 由修改時間與檔案大小組成；不同 Pod 若 build 時間不同，同一個檔案的 ETag 會不同，導致重新下載。 | 映像內的檔案修改時間要固定（可重現的 build），或改用內容 hash 的檔名。 | 為什麼帶 hash 的檔名不需要驗證？ |
| L3 | API 回應能不能用 `proxy_cache`？ | 含使用者資料的回應不能共用快取：cache key 若沒包含身分，使用者 A 會拿到使用者 B 的資料。這個專案的 API 都依登入者過濾，不應快取。 | 就算 key 加上 `Cookie` 也要小心，session 每次登入都變，命中率趨近零。 | 什麼樣的 API 適合快取？ |
| L3 | 下載與 API 回應要設 `Cache-Control: no-store` 嗎？ | 要。避免瀏覽器、共用 proxy 或 CDN 留下他人的簡報與提案資料；與 `nosniff`、CSP 一起由後端 middleware 設定。 | 共用電腦上登出後按「上一頁」仍能看到內容，也是這個原因。 | 怎麼驗證每個回應都帶到 header？ |
| L3 | gzip 或 brotli 要壓哪些類型？ | 壓文字類（HTML、JS、CSS、JSON、SVG）；PDF、PPTX（zip）、圖片本身已壓縮，再壓只浪費 CPU。 | 壓縮含祕密（例如 CSRF token）且攻擊者可控內容的回應，有 BREACH 風險。 | 預壓縮（`gzip_static`）的好處？ |
| L4 | 靜態資源要不要上 CDN？ | 全公司內網使用者時效益有限；若多地點、跨國才明顯。上 CDN 後要處理 cache purge、CSP 的 `script-src` 來源、部署順序，且 CDN 看得到流量。 | 內部系統不要為了「看起來標準」就引入。 | 部署時資源與 `index.html` 的上傳順序？ |

### Header & Conf

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | `add_header` 的繼承規則？ | 子層（`server`、`location`）只要有自己的 `add_header`，就會完全取代上層的，不會合併。 | 在某個 `location` 加了一個 header，CSP 與 HSTS 就消失了，是最常見的安全設定漏洞。 | 怎麼避免這個問題？ |
| L2 | `add_header ... always` 的 `always` 做什麼？ | 預設只在 2xx、3xx 回應加 header；加上 `always` 後 4xx、5xx 也會帶。 | 錯誤頁（404、413、502）沒帶安全 header，仍可能被利用。 | 自訂錯誤頁的 header 怎麼繼承？ |
| L2 | `server_tokens off` 的用途？ | 隱藏錯誤頁與 `Server` header 的 Nginx 版本，減少攻擊者對照已知漏洞的資訊。 | 這只是降低資訊洩漏，不是防禦手段，真正的防線是及時更新。 | 還有哪些 header 會洩漏技術堆疊？ |
| L2 | `nginx -t`、`nginx -s reload` 與重啟的差別？ | `-t` 只檢查語法與引用的檔案；`reload` 啟動新 worker 載入新設定，舊 worker 處理完進行中的連線再結束，不中斷服務；重啟會中斷連線。 | 設定錯誤時 `reload` 會失敗，但舊設定繼續運作；容器重啟後則是起不來。 | 容器啟動前怎麼先檢查設定？ |
| L3 | 改了 Nginx 的 ConfigMap，Pod 沒有 reload？ | 沒有觸發 rollout。Pod template 加上 `checksum/config` annotation，設定一變就滾動更新；或在 sidecar 偵測檔案變動後呼叫 `nginx -s reload`。 | 以 `subPath` 掛載的 ConfigMap 不會自動更新內容。 | 滾動更新與 reload 各自的取捨？ |
| L3 | `client_max_body_size` 可以設在哪些層級？上傳 API 與其他路徑要分開嗎？ | 可設在 `http`、`server`、`location`。全域用較小的值（例如 1m），只在上傳路徑放寬為 6m，縮小被濫用的範圍。ingress-nginx 對應的是 `proxy-body-size` annotation。 | 兩層 Nginx 都要設，只放寬一層，另一層仍會回 413。 | 超過上限時，client 看到什麼？ |
| L3 | 怎麼讓 413、502、504 回 `application/problem+json`？ | 用 `error_page 413 = @problem_413;`，在命名的 `location` 內以 `return 413 '{...}'` 搭配 `default_type application/problem+json`。ingress-nginx 則用 `custom-http-errors` 搭配 default backend。 | 回應中不含 `request_id` 時，可由 Nginx 的 `$request_id` 產生並帶到後端。 | 前端怎麼處理仍然不是 JSON 的錯誤？ |
| L3 | 容器內 `worker_processes auto` 會有什麼問題？ | `auto` 讀的是節點的 CPU 核數，不是容器的 CPU limit。節點 64 核、Pod 只有 0.5 核時，會產生過多 worker 並被 throttle。要明確指定數量。 | `worker_connections` 受檔案描述元上限（`worker_rlimit_nofile`）限制。 | 怎麼看出 Nginx 已經到連線上限？ |
| L3 | TLS 在哪一層終止？後端之間要不要再加密？ | 在 ingress-nginx 終止，叢集內部走明文 HTTP；若合規要求端對端加密，要用 service mesh 的 mTLS 或在後端啟用 TLS。 | session cookie 設了 `Secure` 後，TLS 要在可信的邊界終止並正確設定 `X-Forwarded-Proto`，否則後端判斷 scheme 錯誤。 | HSTS 要設在哪一層？ |

## Deployment & Rollback

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L2 | 怎麼回滾一次部署？ | GitOps 下用 `git revert` 還原映像 tag 的那個 commit，讓 ArgoCD 同步。 | ArgoCD 開啟 auto-sync 與 `selfHeal` 時，手動的 `helm rollback` 或 `kubectl` 會被 Git 的狀態蓋回去。 | 緊急時要不要先停用 auto-sync？ |
| L2 | `helm rollback` 會把 DB schema 一起還原嗎？ | 不會。migration 是單向的，所以舊版程式必須能在新的 schema 上運作，這就是 expand / contract 的目的。 | contract（刪除舊欄位）要等確定不會再回滾之後才執行。 | contract 要在哪一個版本執行？ |
| L3 | migration 和新版程式的上線順序？ | ArgoCD 的 PreSync Job 先執行 expand migration，成功後才滾動更新程式。migration 失敗時同步停止，舊版程式仍能運作。 | migration 不能讓每個 Pod 啟動時各自執行。 | migration 卡在 dirty 狀態怎麼處理？ |
| L3 | 新版有 bug，已經寫入錯誤的資料，回滾程式之後呢？ | 回滾程式不會修正資料。用稽核紀錄找出受影響的列，寫修補腳本或 migration 往前修（fix forward），修補本身也要寫稽核。PITR 會遺失之後所有合法的寫入，是最後手段。 | MinIO 的物件與稽核紀錄都無法回滾。 | PITR 之後，DB 與 MinIO 怎麼對齊？ |
| L3 | 滾動更新時新舊版本同時運作，會有什麼問題？ | API 回應格式、ETag、session 與 CSRF token 的格式都要前後相容。前端舊版的 SPA 可能呼叫新版的 API。 | `index.html` 不能被快取；靜態資源用含 hash 的檔名。 | 前端怎麼得知有新版本？ |
| L3 | 怎麼確認部署成功，失敗時自動回滾？ | readiness 只代表能接流量；部署後用 PostSync hook 跑 smoke test，並觀察錯誤率。Argo Rollouts 可以依 metrics 分析結果自動回滾。 | smoke test 不能寫入正式資料，或要用專屬的測試帳號並能清除。 | 錯誤率上升多少才算失敗？ |
| L4 | canary 或 feature flag 值得導入嗎？ | 使用者少、流量小，canary 的統計意義有限。截止前凍結部署（change freeze）更實際；feature flag 只用在大型功能。 | 引入工具要算進維運成本與團隊學習成本。 | 截止前一週發現嚴重 bug 怎麼辦？ |
| L1 | GitOps 的核心原則？ | 宣告式描述、Git 是唯一事實來源、自動同步、持續比對並修正漂移。 | 與「CI 直接 `kubectl apply`」的差別是：叢集拉取 Git，而不是被推送。 | 拉取式有什麼安全上的好處？ |
| L2 | `auto-sync`、`prune`、`selfHeal` 各自做什麼？ | `auto-sync` 偵測到 Git 變更就同步；`prune` 刪除 Git 已移除的資源；`selfHeal` 把手動改動蓋回 Git 的狀態。 | `prune` 預設關閉，打開前要確認不會刪到不該刪的資源。 | 正式環境要全部打開嗎？ |
| L2 | `Application` 和 `ApplicationSet` 差在哪？ | `Application` 描述一個部署；`ApplicationSet` 用 generator 批量產生，適合多環境、多叢集。 | 用 App of Apps 或 ApplicationSet 管理 ArgoCD 自己的設定。 | 新增一個環境要改哪裡？ |
| L2 | sync wave 和 hook 怎麼用？ | 用 `argocd.argoproj.io/sync-wave` 決定順序（例如 CRD → namespace → 應用）；hook 有 PreSync、Sync、PostSync、SyncFail。 | 同一個 wave 內的資源會等健康後才進入下一個 wave。 | cert-manager 的 CRD 與 `Certificate` 的順序怎麼排？ |
| L3 | 機密怎麼在 GitOps 裡管理？ | 明文不進 Git。用 External Secrets Operator 從 Vault 同步，或用 Sealed Secrets、SOPS 加密後進 Git。 | 加密後的機密仍要管理金鑰與輪替。 | 機密輪替時 ArgoCD 要做什麼？ |
| L3 | cert-manager 產生的 `Secret` 被 ArgoCD 視為 OutOfSync，怎麼辦？ | 不要把產生出來的 `Secret` 放進 Git，只放 `Certificate`；必要時用 `ignoreDifferences` 排除會被改寫的欄位。 | `selfHeal` 開啟時，被改寫的欄位可能一直被蓋回去。 | 還有哪些資源會被控制器改寫？ |
| L3 | 應用程式碼與部署設定要放同一個 repo 嗎？ | 分開。避免 CI 提交映像 tag 又觸發 CI 的迴圈，權限也能分開：開發者改程式碼，只有 Release 能改部署設定。 | 小專案放同一個 repo 較省事，要說明取捨。 | 映像 tag 的變更怎麼自動化？ |
| L3 | 多環境怎麼晉升？ | 用 Kustomize overlay 或 `values-<env>.yaml`，由 Release 對 GitOps repo 開 PR 修改 tag；正式環境要經過審核。 | 環境間只差異必要的值，其餘共用。 | 怎麼確保 staging 驗證過的映像就是 prod 用的那個？ |
| L3 | ArgoCD 自己掛掉會怎樣？ | 已部署的工作負載不受影響，只是停止同步與偵測漂移。要以 HA 方式部署，並備份其設定與 repo 憑證。 | 緊急時要有繞過 ArgoCD 的手動 apply 流程，並事後補回 Git。 | 怎麼監控 ArgoCD 的健康？ |

## Testing

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 單元測試和整合測試怎麼分？ | 單元測試測純邏輯（狀態推算、檢查順序、輸入驗證）；整合測試用 testcontainers 起真的 MariaDB 和 MinIO，測 SQL、約束與交易。 | 不 mock DB 來測 SQL，約束只有真的 DB 才會觸發。 | testcontainers 太慢怎麼辦？ |
| L2 | 覆蓋率 80% 怎麼量？在 CI 怎麼強制？ | `go test -coverpkg=./... -coverprofile=cover.out`，合併單元與整合測試，低於門檻就讓 pipeline 失敗。排除產生的程式碼（oapi-codegen 輸出）。 | 沒有 `-coverpkg` 時，整合測試對其他套件的覆蓋不會被計入。 | 80% 能證明什麼？ |
| L2 | 覆蓋率能說明什麼、不能說明什麼？ | 能說明哪些程式碼完全沒被執行過；不能說明斷言是否正確、邊界情況是否測到。 | 有執行但沒有斷言的測試也會拉高覆蓋率。 | 你怎麼確認測試品質？ |
| L3 | 樂觀鎖怎麼測？ | 兩個 goroutine 讀到同一個 ETag，同時更新；斷言恰好一個成功、另一個得到 412，且 version 只加 1。 | testcontainers 通常只起單節點 MariaDB，測不到 Galera 的 certification 行為，要說明這個缺口。 | 怎麼補上 Galera 的測試？ |
| L3 | 稽核不變量怎麼測？ | 對每個寫入 API 斷言「成功就一定有對應稽核紀錄」。另外注入稽核寫入失敗（例如暫時撤銷權限或用故障的 repository），斷言業務資料也沒有寫入。 | 也要測被拒絕的請求有留下紀錄。 | 怎麼確保新 API 不會漏測？ |
| L3 | 防枚舉怎麼測？ | 表格驅動測試：「不存在」與「無權限」的回應狀態碼與 body 完全相同。計時差異用多次取樣比較分布，不用單次時間。 | 計時測試在 CI 容易不穩定，可只在特定環境執行。 | 怎麼避免 flaky test？ |
| L3 | 怎麼確保實作沒有偏離 API 規格？ | CI 重新執行 oapi-codegen 並比對 diff；整合測試用 kin-openapi 驗證實際的請求與回應符合 `openapi.yaml`。 | 產生的 interface 只保證型別，不保證狀態碼與 header。 | 前端怎麼跟上規格變更？ |
| L3 | 錯誤分支（1062、1213、MinIO timeout）怎麼測到？ | 用會回傳錯誤的 repository 或 storage 實作注入錯誤；用 Toxiproxy 製造延遲；暫時撤銷 DB 權限讓稽核寫入失敗。 | 錯誤分支是覆蓋率最常缺的地方，也最容易在正式環境出事。 | 單節點 MariaDB 怎麼重現 1213？ |
| L4 | 測試策略怎麼分配投資？ | 依風險分配：授權、截止、稽核、一致性是高風險，要有整合測試與不變量測試；UI 細節以少量 E2E 覆蓋關鍵流程。 | 能說出「刻意不測什麼、為什麼」。 | 上線後怎麼發現測試沒抓到的問題？ |

## Frontend

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L1 | 前端怎麼顯示截止狀態？ | 以後端回傳的狀態與 server 時間為準，不用瀏覽器的本地時間判斷。 | 倒數計時可用 server 時間與本地時間的差值校正。 | 截止瞬間使用者正在編輯怎麼辦？ |
| L2 | 收到 412 時，前端怎麼處理？ | 告訴使用者資料已被他人修改，提供重新載入最新版本的選項，並保留使用者尚未送出的內容讓他比對。 | 不要自動覆蓋，會丟失別人的修改。 | 收到 428 代表什麼？ |
| L2 | 上傳 5 MiB 檔案的體驗怎麼做？ | 送出前先在前端檢查大小與副檔名（只為體驗，後端仍要驗證），顯示上傳進度，失敗時保留表單內容。 | `fetch` 不提供上傳進度，需要用 `XMLHttpRequest`。 | 503 時前端顯示什麼？ |
| L2 | 收到 503 時，前端顯示什麼？ | 說明檔案服務暫時無法使用，保留表單內容，依 `Retry-After` 提供重試；列表與詳情照常顯示。 | 要讓使用者知道資料並未寫入，不能只顯示「失敗」。 | 要不要自動重試？ |
| L3 | SPA 怎麼取得 CSRF token 與處理 session 逾時？ | 登入後由 API 回傳 token 或從 cookie 讀取，寫入請求帶自訂 header。收到 401 時導回登入頁並保留原本路徑。 | 閒置逾時發生在使用者填表途中會丟資料，可在前端提示即將逾時。 | 多個分頁同時開啟時怎麼處理？ |
| L3 | 多個分頁同時開啟，一個分頁登出或逾時，其他分頁怎麼辦？ | 用 `BroadcastChannel` 或 `storage` 事件同步登出；任何分頁收到 401 都導回登入頁。 | session 仍放在 HttpOnly cookie，不要為了同步把 token 存進 localStorage。 | 重新登入後，其他分頁要自動恢復嗎？ |

## Language-Specific (Ecosystem & Syntax & Framework)

> 主檔的題目與語言無關。本節只收「同一個概念在不同語言或框架下的預設行為與陷阱」。Category 分為 Ecosystem、Syntax、Framework、Runtime。

### 概念對照

| 概念 | Go | Java / Spring Boot | TS / Fastify / Drizzle |
| --- | --- | --- | --- |
| 檢查順序 | middleware 鏈 | Security filter chain | hooks lifecycle |
| 交易 | 明確傳遞 `*sql.Tx` | `@Transactional`（proxy） | `db.transaction(async (tx) => …)` |
| 樂觀鎖 | `WHERE version = ?` 加 `RowsAffected` | `@Version` | 手寫 `where` 加 `affectedRows` |
| request-id 傳遞 | `context` 加 `slog` | MDC | `genReqId` 加 pino child logger、`AsyncLocalStorage` |
| multipart 串流 | `MultipartReader` | Servlet multipart | `@fastify/multipart` |
| Migration | golang-migrate | Flyway | drizzle-kit |
| API Spec | oapi-codegen | springdoc、openapi-generator | `@fastify/swagger`、openapi-typescript |
| 覆蓋率 | `go test -coverpkg` | JaCoCo | Vitest（v8 或 istanbul） |
| bcrypt | `golang.org/x/crypto/bcrypt` | `BCryptPasswordEncoder` | `bcrypt`（原生模組） |
| distroless 映像 | `static` | `java21` | `nodejs22` |

### Go

| Levels | Category | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- | --- |
| L1 | Syntax | 怎麼判斷 DB 錯誤是 1062？ | 用 `errors.As` 取出 `*mysql.MySQLError`，比對 `Number == 1062`。包裝錯誤時用 `%w`，才能沿著錯誤鏈判斷。 | 用 `%v` 包裝會斷開錯誤鏈。 | 怎麼知道衝突的是哪一個唯一鍵？ |
| L2 | Ecosystem | MySQL driver 的 DSN 要設哪些參數？ | `parseTime=true`、`loc=UTC`、`clientFoundRows=true`，並在連線時設定 `time_zone='+00:00'`。 | 少了 `clientFoundRows`，資料沒有變動時 `RowsAffected` 為 0，會被誤判成 412。 | 連線池怎麼設？ |
| L2 | Framework | 用標準庫的 `ServeMux` 還是 chi？ | Go 1.22 起，`ServeMux` 支援 method 與路徑參數，可以搭配 oapi-codegen 的 strict server。chi 的 middleware 生態較完整。 | 選擇要和 oapi-codegen 支援的 router 一致。 | middleware 的順序怎麼對應檢查順序？ |
| L2 | Syntax | client 斷線時，`context` 會發生什麼事？ | request 的 ctx 被取消，進行中的 DB 查詢與 MinIO 呼叫一起中止。 | 「被拒絕的稽核紀錄」也會跟著寫入失敗，要用 `context.WithoutCancel` 另外寫入。 | 下載途中斷線，稽核怎麼記？ |
| L2 | Runtime | `http.Server` 要設哪些 timeout？ | `ReadHeaderTimeout` 防 slowloris，再設 `ReadTimeout`、`IdleTimeout`。`WriteTimeout` 要容納 5 MiB 的下載，或用 `http.ResponseController` 針對單一請求調整。 | 預設值是 0，代表沒有 timeout。 | 網路慢的使用者上傳會被誤殺嗎？ |
| L3 | Runtime | 容器內的 `GOMAXPROCS` 與記憶體怎麼設？ | Go 1.25 起，`GOMAXPROCS` 預設會依 cgroup 的 CPU limit 設定。記憶體用 `GOMEMLIMIT` 設為 limit 的 80–90%，讓 GC 在 OOM 之前回收。 | 舊版本需要 `automaxprocs` 套件。 | 怎麼觀察 GC 對 p95 的影響？ |
| L3 | Runtime | 50 個人同時上傳 5 MiB，記憶體怎麼估？ | 用 `MultipartReader` 直接串流到 MinIO 的 `PutObject`，不把整個檔案放進記憶體；用 semaphore 限制同時上傳數。驗證檔頭只需 `bufio.Reader.Peek` 前幾 KB。 | multipart 的檔案 part 沒有自己的長度；大小未知時 minio-go 會依 `PartSize` 緩衝，要明確設定。 | 要不要先寫到暫存檔？ |
| L3 | Syntax | 背景排程的 goroutine 怎麼避免洩漏，並隨程式關閉？ | 用 `errgroup` 搭配根 context；收到 SIGTERM 時取消 context，再等待所有工作結束。 | 用 `time.Ticker` 時記得 `Stop()`。 | 怎麼測 goroutine 洩漏（例如 goleak）？ |
| L3 | Ecosystem | `-race` 在 CI 怎麼跑？ | 測試時開 `-race` 抓 data race。 | `-race` 需要 cgo，和 `CGO_ENABLED=0` 的正式 build 要分開執行。 | race 只在並發時才會觸發，怎麼提高抓到的機率？ |
| L3 | Ecosystem | `govulncheck` 和 Whitesource 差在哪？ | `govulncheck` 做可達性分析，只回報程式實際呼叫到的漏洞函式；SCA 只看相依清單。 | 兩者互補，不能互相取代。 | 漏洞不可達時要不要升級？ |
| L4 | Ecosystem | 這個專案為什麼選 Go？ | 編譯成單一靜態執行檔，可用 distroless/static；並發模型適合串流 I/O；標準庫就足夠。代價是 ORM 與框架生態較薄，SQL 要自己寫。 | 要能說出換成 Java 會失去什麼、得到什麼。 | 團隊沒有 Go 經驗呢？ |

### Java / Spring Boot

| Levels | Category | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- | --- |
| L1 | Ecosystem | Spring Boot 的 starter 與 BOM 解決什麼問題？ | starter 打包常用的相依；BOM 統一管理版本，避免版本衝突。 | 覆寫 BOM 管理的版本要有理由並記錄。 | 怎麼找出間接相依的 CVE？ |
| L2 | Framework | 未登入又沒帶 CSRF token 的 POST，Spring Security 回什麼？ | 回 403。預設的 `CsrfFilter` 排在認證之前，和規格的 401 → CSRF 順序相反，要調整 filter 順序或自訂處理。 | 這是框架預設和規格衝突的典型例子。 | 怎麼用測試鎖住檢查順序？ |
| L2 | Framework | `@Transactional` 什麼時候不會生效？ | 同一個類別內部呼叫（self-invocation）不經過 proxy；非 public 方法；checked exception 預設不會 rollback。 | 被拒絕的稽核紀錄要用 `REQUIRES_NEW` 另開交易。 | 怎麼確認交易真的有開？ |
| L2 | Framework | 1062 在 Spring 會變成什麼例外？ | `DataIntegrityViolationException`，要往下取出 `SQLIntegrityConstraintViolationException` 的錯誤碼與約束名稱。 | 不要用例外訊息的字串比對作為主要判斷。 | 怎麼對應到 API Error Contract 的錯誤碼？ |
| L2 | Ecosystem | JaCoCo 有哪些 counter？報告的紅、黃、綠怎麼看？ | 六種：INSTRUCTION、BRANCH、LINE、COMPLEXITY、METHOD、CLASS。分支的菱形：紅色是完全沒走到，黃色是部分走到，綠色是全部走到。 | 門檻要設在 BRANCH，只看 LINE 會高估。 | 單元與整合測試的報告怎麼合併？ |
| L3 | Ecosystem | JaCoCo 的分支覆蓋等於 condition coverage 嗎？ | 不等於。JaCoCo 量的是 bytecode 的分支，不是 condition 或 MC/DC。編譯器產生的分支（例如 try-with-resources）有一部分會被自動過濾。 | 排除 Lombok 要設 `lombok.addLombokGeneratedAnnotation=true`；openapi-generator 的輸出也要排除。 | 用 `jacoco:check` 設門檻的 rule 怎麼寫？ |
| L3 | Framework | 用 JPA `@Version` 實作 If-Match，常見的 bug 是什麼？ | 先 load entity 再修改，比對的是剛讀出來的版本，不是客戶端送來的 If-Match，樂觀鎖等於沒做。要先手動比對 If-Match，或把客戶端的版本設進 entity。 | 衝突時的例外是 `ObjectOptimisticLockingFailureException`，要轉成 412。 | 為什麼不用 `@Lock(PESSIMISTIC_WRITE)`？ |
| L3 | Framework | 提案列表會遇到哪些 JPA 效能問題？ | N+1 查詢；`open-in-view` 預設開啟，讓 lazy loading 延伸到 view 層；fetch join 加分頁時，Hibernate 會在記憶體內分頁並發出警告（Hibernate 5 為 HHH000104）。 | 列表 API 的 RPS 驗收很容易被這幾項拖垮。 | 用 DTO projection 還是 entity？ |
| L3 | Runtime | 用 virtual threads 有什麼要注意？ | JDK 21–23 在 `synchronized` 區塊內會 pinning，JDK 24 的 JEP 491 修正了這點。真正的上限是 HikariCP 的連線數，不是執行緒數。 | 不要因為有 virtual threads 就把連線池開大。 | 和 Go 的 goroutine 有什麼差別？ |
| L3 | Ecosystem | request-id 用 MDC 傳遞，會在哪裡遺失？ | MDC 是 ThreadLocal，跨 `@Async`、`CompletableFuture` 或其他執行緒池時會遺失，要用 `TaskDecorator` 複製過去。 | Micrometer Tracing 可以自動傳遞。 | 背景排程的 job id 怎麼帶？ |
| L3 | Runtime | JVM 在容器內的記憶體與啟動時間？ | 用 `-XX:MaxRAMPercentage` 依容器的 limit 設定 heap；heap 以外的 metaspace、執行緒堆疊也要算進去。啟動慢就加 `startupProbe`。 | 把 `-Xmx` 設成等於 limit，會被 OOMKilled。 | 要不要用 CDS 或 GraalVM native image？ |
| L3 | Framework | multipart 的上限與暫存檔怎麼設？ | `spring.servlet.multipart.max-file-size`、`max-request-size`、`file-size-threshold`、`location`。 | 超過 threshold 會寫暫存檔，distroless 搭配唯讀根檔案系統時要掛 `emptyDir`。 | 怎麼改成串流處理？ |
| L4 | Ecosystem | 同樣的需求用 Spring Boot 做，值得嗎？ | 生態完整（Security、JPA、Actuator），團隊熟悉時開發快。代價是映像較大、啟動較慢，而且框架的預設行為要逐一確認是否符合規格。 | 「預設行為和規格衝突」的地方要列成清單。 | 你會關掉哪些 auto-configuration？ |

### TypeScript / Fastify / Drizzle

| Levels | Category | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- | --- |
| L1 | Syntax | 有了 TS 型別，為什麼還要驗證 request？ | 型別在編譯後就被抹除，執行期不存在。request 要用 TypeBox 或 Zod 做執行期驗證。 | 用 `as` 斷言型別等於跳過檢查。 | 型別和 schema 怎麼只維護一份？ |
| L2 | Framework | Fastify 的 hook 順序會和規格的檢查順序衝突嗎？ | 會。順序是 onRequest → preParsing → preValidation → validation → preHandler → handler。授權若放在 preHandler，schema 驗證會先執行，422 就排到 404 前面。授權要放在 onRequest 或 preValidation。 | Fastify 驗證失敗預設回 400 不是 422，要自訂 error handler。 | 怎麼測試檢查順序？ |
| L2 | Ecosystem | `bcrypt` 還是 `bcryptjs`？ | `bcryptjs` 是純 JS，會阻塞 event loop。原生的 `bcrypt` 跑在 libuv threadpool，預設只有 4 條執行緒（`UV_THREADPOOL_SIZE`）。 | 登入尖峰加上 dummy bcrypt 會在 threadpool 排隊，也會拖慢同樣使用 threadpool 的檔案 I/O。 | 怎麼監控 event loop lag？ |
| L2 | Ecosystem | Drizzle 的交易最常見的 bug？ | 在 `db.transaction(async (tx) => …)` 裡誤用 `db`，那筆寫入就在交易之外，rollback 不到。 | 讓 repository 必須傳入執行者（`db` 或 `tx`），讓這個錯誤無法發生。 | 跨模組怎麼共用交易？ |
| L2 | Ecosystem | Drizzle 怎麼做樂觀鎖與參數化？ | `where(and(eq(proposals.id, id), eq(proposals.version, version)))`，再檢查 `affectedRows`。`sql` template 會自動參數化，`sql.raw` 不會。 | 要確認 driver 的 FOUND_ROWS 旗標，原因和 Go 的 `clientFoundRows` 相同。 | `ORDER BY` 的欄位怎麼做白名單？ |
| L2 | Ecosystem | bigint id 和 `DATETIME` 在 Node 會出什麼問題？ | bigint 超過 2^53 會失去精度，Drizzle 要選 `mode: 'bigint'` 或字串。mysql2 預設以本地時區解析 `DATETIME`，要設 `timezone: 'Z'`。 | `BigInt` 無法直接 `JSON.stringify`。對外只露出 uuid 也能避開這個問題。 | `BINARY(16)` 的 uuid 怎麼轉換？ |
| L2 | Ecosystem | `drizzle-kit push` 和 `migrate` 差在哪？ | `push` 直接把 schema 同步到 DB，不產生 migration 檔，只適合本機開發。正式環境用 `generate` 產生 SQL，經過 review 再 `migrate`。 | 產生的 SQL 要確認 collation 與 `CHECK` 有沒有正確輸出。 | migration 由誰執行？ |
| L3 | Runtime | multipart 上傳與下載的串流要注意什麼？ | 用 `stream.pipeline` 處理 backpressure 與錯誤。`@fastify/multipart` 的檔案串流必須讀完或丟棄，否則請求會卡住；用 `limits.fileSize` 限制大小。 | 用 `toBuffer()` 會把整個檔案載入記憶體。 | 怎麼邊上傳邊驗證檔頭？ |
| L3 | Ecosystem | request-id 怎麼帶到每一行日誌？ | 用 Fastify 的 `genReqId` 產生；`request.log` 是 pino 的 child logger，會自動帶上。拿不到 request 的深層程式用 `AsyncLocalStorage`。 | 外部傳入的 header 要驗證格式。 | 背景排程呢？ |
| L3 | Runtime | 未處理的 promise rejection 會怎樣？怎麼優雅關閉？ | Node 15 起預設會讓 process 結束。關閉時呼叫 `fastify.close()`，等待進行中的請求，再關閉 DB 連線池。 | 漏寫 `await` 是 rejection 沒被處理的常見原因，用 typescript-eslint 的 `no-floating-promises` 擋下。 | SIGTERM 到 process 結束之間要做什麼？ |
| L3 | Syntax | CSRF token 怎麼比對？ | 用 `crypto.timingSafeEqual`，兩邊長度要先確認相同，否則會拋出例外。 | 直接用 `===` 有計時側通道。 | 為什麼 session 查詢不需要這樣做？ |
| L3 | Ecosystem | npm 供應鏈的風險怎麼控制？ | 提交 lockfile；CI 用 `npm ci --ignore-scripts`，避免 postinstall 執行惡意程式；SCA 掃描；必要時使用私有 registry。 | npm 的間接相依數量遠多於 Go 與 Java。 | typosquatting 怎麼防？ |
| L3 | Ecosystem | Vitest 的 v8 與 istanbul 覆蓋率差在哪？ | v8 用引擎內建的覆蓋率，速度快；istanbul 在原始碼插樁，分支計算較精確。 | 兩者的數字不能直接比較，換工具時門檻要重新校準。 | 門檻設在 branch 還是 line？ |
| L4 | Ecosystem | 前後端都用 TS 的 monorepo，值得嗎？ | 型別與 schema 可以共用，契約變更時編譯就會報錯。代價是建置工具較複雜，前後端的發布節奏也會綁在一起。 | 共用型別不能取代 OpenAPI，外部呼叫者仍然需要規格。 | 共用的套件怎麼做版本管理？ |

## Behavioral & Ownership

| Levels | Questions | Recommended Answer | Notice | Follow-up |
| --- | --- | --- | --- | --- |
| L2 | 規格中有哪些模糊或矛盾？你怎麼處理？ | 舉實例：PTPX 筆誤、Admin 與 state 403 的關係、成員能否被移除、「替管理員建立簡報」的語意。做法是列出問題、提出假設、和需求方確認、寫回規格。 | 重點是「怎麼確認」，不是「自己猜」。 | 需求方不回應時怎麼辦？ |
| L3 | 遇過最難的 bug 是什麼？ | 用「現象、假設、驗證、根因、修正、防止再發生」的結構回答，帶出實際的數字與工具。 | 不要只說修好了，要說學到什麼、補了什麼測試。 | 再遇到一次，你多久能找到？ |
| L3 | 哪個需求你會反對？ | 例如：自行註冊無法驗證身分、Admin 豁免截止有公平性風險。提出替代方案與成本。 | 反對要附替代方案，否則只是抱怨。 | 對方堅持時呢？ |
| L4 | 多兩週你會做什麼？時間只剩一半會砍什麼？ | 多兩週：補強風險最高的地方，例如備份還原演練、Galera 的並發測試。時間減半：保留授權、稽核、一致性，先砍加分功能與 UI 細節。 | 優先順序要對應到風險，不是個人喜好。 | 這個專案最大的技術債是什麼？ |
