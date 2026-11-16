# ESG Idea Contest

## 產品功能

- 公司內部的IT員工ESG點子競賽平台
- 一個團隊=一個Leader + 最多4位Member 對應一個Proposal (提案)
- 每一個Proposal需要附上一個簡報檔案 (PPTX, PDF)
- 管理員可以編輯與檢視所有提案的檔案，而管理員本身可以是提案者或組員

## 模組

- 帳號：用6碼工號 姓名 與密碼自行註冊 不需要Email，Admin由Vault注入與同步
- 登入：
  - Session存在Server-side MariadDB，只存ID SHA-256
  - 閒置30 min, 最長 8 小時，
  - 登入連續失敗五次鎖定15分鐘
  - 同一IP 15分鐘內失敗30次會被throttle
- 提案：
  - 建立時送一個multipart
  - 請求時同時送出自料與簡報，具備原子性
  - 編輯是整筆替換
  - 一定要帶ETag / If-Match 做樂觀鎖
  - 同一個Leader的提案名稱不可以重複
  - 支援分頁(最多100筆) 篩選 排序
- 團隊成員：Leader 用工號加組員，成員只能檢視與下載簡報，不能自行退出
- 簡報
  - 只支援PDF, PTPX
  - 上限5 MiB
  - 有預期檔案名稱
  - 重傳會覆蓋舊檔
  - 刪除提案會刪除檔案
  - 下載由後端串流 不用Presigned URL 或是分塊串流
- 截止時間：
  - 狀態根據資料庫時間即時 推算(NOT OPEN, OPEN, CLOSED)
  - 截止後 使用者的寫入一律擋下寫入 但檢視與下載不受影響
  - Admin不受截止時間影響
- Admin:
  - 可以設定截止時間
  - 可以替管理員建立編輯簡報
  - 可以檢視管理所有提案與提案者
  - 可以查詢Audit Log
- Audit Log
  - 紀錄註冊 登入 提案寫入 下載與被拒絕的嘗試
  - 寫入失敗時會整筆操作一起失敗
  - 不會有沒留下稽核紀錄的成功寫入

### 加分功能

- 統計儀表板
- Excel匯出
- Audit hash chain
- 孤兒檔案排程

## 基本需求

- 後端測試覆蓋率 >= 80%
- 完成Azure CI Pipeline 需要包含：Whitesource, Semgrep, Base Image Scan, Secret Detect等安全掃描
- 透過Azure Release + ArgoCD GitOps + Helm 部署到 K8S
- 獲取提案的API 在10,000筆檔案下，RPS >= 100 p95 < 300ms
- 建立API-Spec
- Structured Logging: JSON Log 每行都需要帶入request-id
- Fault Tolerance: MinIO 掛掉時刪除仍會成功 上傳與下載回503
- 密碼與Session只存Hash
- 密碼用Bcrypt cost 10 + Session 用 SHA-256
- 所有查詢需一律參數化 有分頁上限
- 無權限一律顯示404 並用dummy bcrypt讓回應時間一致避免被枚舉
- 加上CSP在Security Header

## 技術

- Golang 1.26
- React + TS
- MariaDB Galera 10.5
- MinIO
- K8S
- oapi-codegen
- golang-migrate
- distroless base image
- testcontainer-go

## 架構

Browser (Client) -> Ingress -> Frontend (Nginx + SPA) -> API -> Backend -> MariaDB  （所有狀態, Session, 計數器, 時間）
                                                          -> MinIO
                                                          -> Vault

- 採用Module monlith區分：admin, auth, account, proposal, storages, slides, event, audit, shared
- 如果能透過資料庫約束就交給資料庫
- 使用雙id策略 對外一律uuid, 對內一律bigint id加速檢索
- 唯一的時鐘是資料庫時間 UTC+0
- 檢查順序固定： 401 -> CSRF -> 404 -> role 403 -> 428/412 -> state 403 -> 422
- 儲存一致性：每次上傳或寫入都寫new object key 被取代或刪除的檔案紀錄到 pending_file_deletion 與資料異動在同一個transaction 之後再非同步清除
- 前後端同源 不需要CORS

## 對應章節

> 章節編號指各領域目錄（`docs/contents/<領域>.md`）的節。已寫成文章的附連結，其餘為目錄中的條目，尚未成文。

### 帳號與登入

| 專案需求 | 對應章節 |
| --- | --- |
| 密碼 Bcrypt cost 10 | [密碼雜湊](../site/src/content/docs/backend/l1/1-4-auth-basics/password-hashing.md) |
| Session 只存 SHA-256、閒置與絕對逾時 | [Session 與 Cookie](../site/src/content/docs/backend/l1/1-4-auth-basics/session-cookie.md)、[Session vs Token](../site/src/content/docs/backend/l1/1-4-auth-basics/session-vs-token.md)；後端 2.6「Session 存資料庫」 |
| 認證與授權的分工 | [authn vs authz](../site/src/content/docs/backend/l1/1-4-auth-basics/authn-vs-authz.md) |
| 登入失敗鎖定、IP throttle | 後端 2.6「登入鎖定與限速」、2.5「限流算法」；資安 2.4「暴力破解防護」 |
| 無權限回 404、dummy bcrypt 防枚舉 | 後端 2.6「帳號枚舉防護」；資安 2.4「帳號枚舉與計時側通道」「以 404 取代 403」 |
| Admin 由 Vault 注入 | 維運 2.3「HashiCorp Vault」；後端 2.6「不使用 Email 的帳號體系」 |

### 提案、團隊與簡報

| 專案需求 | 對應章節 |
| --- | --- |
| multipart 同時送出資料與簡報、具備原子性 | 後端 4.1「資料與檔案同一個請求」「跨系統一致性」 |
| ETag / If-Match 樂觀鎖（412 / 428） | 後端 2.2「樂觀鎖的 SQL 實作」、2.5「條件請求與樂觀並發控制」；測試 2.3「並發與樂觀鎖測試」 |
| 同一 Leader 提案名稱不可重複 | 後端 2.2「讓資料庫守住不變量」（複合 `UNIQUE`） |
| 分頁（上限 100）、篩選、排序 | 後端 2.5「分頁的上限與排序穩定性」、2.2「分頁（offset vs cursor）」 |
| Leader / Member 權限 | 後端 4.2「團隊與成員」「資源層級授權」 |
| PDF / PPTX 驗證、5 MiB 上限 | 後端 4.1「檔案驗證」；資安 2.4「上傳檔案的驗證層次」 |
| 後端串流下載、不用預簽名 URL | 後端 4.1「後端串流下載」「上傳的三種做法」 |
| MinIO | 後端 4.1「物件儲存實務」 |
| 新 object key、`pending_file_deletion`、孤兒檔案排程 | 後端 4.1「跨系統一致性」「中繼資料與物件分離」、2.4「資料庫驅動的待辦表」 |

### 截止時間與時鐘

| 專案需求 | 對應章節 |
| --- | --- |
| 唯一時鐘是資料庫時間 UTC+0 | 後端 2.2「以資料庫時間作為唯一時鐘」 |
| 狀態即時推算 NOT OPEN / OPEN / CLOSED | 後端 2.2「以資料庫時間作為唯一時鐘」；前端 3.6「截止時間與狀態」 |
| 檢查順序 401 → CSRF → 404 → 403 → 428/412 → 403 → 422 | 後端 2.5「錯誤判斷的固定順序」；[CSRF](../site/src/content/docs/cyber/p2-web/2-3-cross-site/csrf.md)；測試 2.3「授權與防枚舉測試」 |

### 稽核與統計

| 專案需求 | 對應章節 |
| --- | --- |
| 稽核失敗整筆失敗、不漏記 | 後端 4.3「失敗即整筆失敗」「資料模型與寫入方式」；測試 2.3「稽核不變量測試」 |
| Audit hash chain | 後端 4.3「防竄改」 |
| 統計儀表板、Excel 匯出 | 後端 4.8「統計儀表板」「Excel 匯出」；前端 3.5「資料密集的畫面」 |

### 架構與資料庫

| 專案需求 | 對應章節 |
| --- | --- |
| 模組化單體 | 後端 3.3「單體、模組化單體、微服務」「Go 模組化單體實作」 |
| 雙 id 策略（uuid 對外、bigint 對內） | [主鍵選型](../site/src/content/docs/backend/l2/2-2-database/primary-key-choice.md) |
| MariaDB Galera | [MariaDB 標準版與 Galera 叢集](../site/src/content/docs/backend/l3/3-1-distributed-basics/mariadb-galera.md) |
| golang-migrate | 後端 1.3「Migration」「用 golang-migrate 管理 migration」（Java 版見「用 Flyway 管理 migration」） |
| 資料庫約束優先 | 後端 2.2「約束」「讓資料庫守住不變量」 |
| oapi-codegen、API Spec | 後端 2.5「OpenAPI 規格與文件」「規格先行與程式碼生成」 |
| MinIO 掛掉時刪除成功、上傳下載回 503 | 後端 3.5「依賴故障時的部分可用」；測試 4.3「依賴中斷的測試」 |
| 前後端同源、不需 CORS | [CORS](../site/src/content/docs/backend/l1/1-4-auth-basics/cors.md)、[同源政策](../site/src/content/docs/cs/n-network/n-4-application/same-origin-policy.md) |
| Structured Logging、request-id | 後端 2.8「結構化日誌與 correlation ID」；維運 3.4「JSON 結構化日誌的蒐集」 |

### 安全

| 專案需求 | 對應章節 |
| --- | --- |
| CSP 與 Security Header | [XSS](../site/src/content/docs/cyber/p2-web/2-3-cross-site/xss.md)（CSP nonce）；資安 2.3「安全回應標頭」；前端 4.2、4.5 |
| 所有查詢參數化 | 後端 1.3「防止 SQL injection」 |
| 供應鏈與機密偵測 | 資安 2.6「供應鏈與機密洩漏的偵測」；維運 1.3「安全掃描閘門」 |

### 部署與 CI

| 專案需求 | 對應章節 |
| --- | --- |
| distroless base image | [distroless](../site/src/content/docs/devops/l1/1-2-container-image/distroless-image.md)、[golden image](../site/src/content/docs/devops/l1/1-2-container-image/golden-image.md)、[容器映像攻擊面](../site/src/content/docs/cyber/p4-advanced/4-3-cloud-container/container-image-attack-surface.md) |
| Azure CI：Whitesource、Semgrep、Base Image Scan、Secret Detect | 維運 1.3「Azure Pipelines」「安全掃描閘門」 |
| Azure Release + ArgoCD + Helm 部署到 K8s | 維運 3.2「寫一個自己的 chart」「Release 與 GitOps 銜接」、3.3「ArgoCD 實作」、3.1「有狀態服務與儲存」 |
| Nginx + SPA、Ingress | [Nginx 負載平衡 1/3](../site/src/content/docs/backend/l3/3-4-traffic-entry/nginx-lb-selection.md)；維運 3.1「Ingress 與同源前後端」；前端 4.5「建置與部署」 |

### 測試與效能

| 專案需求 | 對應章節 |
| --- | --- |
| testcontainers-go | [Testcontainers for Go](../site/src/content/docs/testing/l2/2-2-integration-testing/testcontainers-go.md)；測試 2.2「Testcontainers 搭 MariaDB 與 MinIO」 |
| 後端覆蓋率 >= 80% | 測試 3.4「覆蓋率能說明什麼」「在 CI 強制覆蓋率門檻」 |
| 一萬筆下 RPS >= 100、p95 < 300ms | 後端 3.6「列表 API 的效能驗收」；測試 4.1「以一萬筆資料驗收列表 API」 |

### 前端

React + TypeScript 對應前端領域（`docs/contents/frontend.md`）：認證 3.2、檔案 3.4、資料密集畫面 3.5、並發衝突 3.6、測試 4.1、安全 4.2。

### 規格備註

- 「簡報」一節的「PTPX」疑為「PPTX」的筆誤。
- 「Admin 不受截止時間影響」與檢查順序中的「state 403」需要明確交代：Admin 略過 state 檢查，一般使用者在截止後被擋。
