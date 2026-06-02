# VoteLedger Chaincode — Tài liệu hàm chức năng

Chaincode Hyperledger Fabric viết bằng Go, triển khai sổ cái bỏ phiếu mù bất biến. Tên contract: `VoteLedgerContract`.

## Mục lục

1. [Tổng quan](#1-tổng-quan)
2. [Cấu trúc state trên ledger](#2-cấu-trúc-state-trên-ledger)
3. [Hàm Vote Contract](#3-hàm-vote-contract)
   - 3.1 [SubmitVote](#31-submitvote)
   - 3.2 [GetVote](#32-getvote)
4. [Hàm Merkle Contract](#4-hàm-merkle-contract)
   - 4.1 [CommitMerkleRoot](#41-commitmerkleroot)
   - 4.2 [GetMerkleRoot](#42-getmerkleroot)
   - 4.3 [VerifyVoteReceipt](#43-verifyvotereceipt)
   - 4.4 [GetAuditCounts](#44-getauditcounts)
5. [Hàm Reveal Contract](#5-hàm-reveal-contract)
   - 5.1 [RevealVoteCompact](#51-revealvotecompact)
   - 5.2 [GetTally](#52-gettally)
   - 5.3 [GetUsedReveal](#53-getusedreveal)
   - 5.4 [ComputeRevealPayloadHash](#54-computerevealpayloadhash)
6. [Key schema trên ledger](#6-key-schema-trên-ledger)
7. [Luồng trạng thái election trên chain](#7-luồng-trạng-thái-election-trên-chain)
8. [Kiểu dữ liệu trả về (Response types)](#8-kiểu-dữ-liệu-trả-về-response-types)
9. [Thiết kế tránh MVCC conflict](#9-thiết-kế-tránh-mvcc-conflict)

---

## 1. Tổng quan

Chaincode không lưu trạng thái election (PENDING/ACTIVE/CLOSED), không quản lý user, không thực hiện xác thực danh tính. Trách nhiệm của chaincode là **ghi nhận sự kiện bất biến** và **phục vụ query kiểm chứng**:

| Sự kiện | Hàm invoke |
|---|---|
| Voter nộp phiếu mù | `SubmitVote` |
| Admin đóng election (commit Merkle root) | `CommitMerkleRoot` |
| Voter giải mù phiếu | `RevealVoteCompact` |

Chaincode chạy ở mode **external chaincode** (không nhúng vào peer), lắng nghe tại `0.0.0.0:9999` theo giá trị `CHAINCODE_SERVER_ADDRESS`.

---

## 2. Cấu trúc state trên ledger

Tất cả state được lưu bằng composite key trong World State của Fabric. Chỉ có **hai loại record ghi** (mỗi record có key riêng biệt per vote/per reveal); không có counter dùng chung.

| Prefix | Loại key | Mô tả |
|---|---|---|
| `vote` | `vote\nelectionId\nvoteId` | Bản ghi phiếu bầu — nguồn sự thật cho `totalVoteCount` |
| `root` | `root\nelectionId` | Merkle root của election |
| `usedReveal` | `usedReveal\nelectionId\nrevealKeyBase64URL` | Khóa reveal đã dùng, lưu kèm `candidateIdsJSON` và `payloadHash` — nguồn sự thật cho tally và `revealCount` |

> **Không có `stats` hay `tally` counter.** Tổng số phiếu, số reveal, và kết quả theo ứng viên đều được tổng hợp on-demand bằng range query (`GetStateByPartialCompositeKey`) trực tiếp trên record gốc. Thiết kế này loại bỏ hoàn toàn MVCC_READ_CONFLICT khi nhiều voter vote/reveal đồng thời — xem [mục 9](#9-thiết-kế-tránh-mvcc-conflict).

---

## 3. Hàm Vote Contract

### 3.1 SubmitVote

**Loại:** `invoke` (write transaction)

**Mục đích:** Ghi nhận phiếu bầu mù lên ledger. Được gọi khi voter nộp phiếu.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |
| `voteID` | string | ID duy nhất của phiếu (ObjectId hex, do backend sinh) |
| `blindedCommitment` | string | SHA256 hex 64 chars của điểm C' (phiếu mù của voter) |

**Logic thực hiện:**

```
1. Validate: electionID, voteID không rỗng
2. Parse blindedCommitment thành [32]byte (phải là valid hex 64 chars)
3. Kiểm tra election chưa commit Merkle root (loadMerkleRoot → root.Committed = false)
   → Nếu đã committed: từ chối (election đã đóng, không nhận phiếu mới)
4. Kiểm tra vote chưa tồn tại (composite key vote\nelectionID\nvoteID)
   → Nếu đã tồn tại: từ chối (chặn duplicate voteID)
5. Tạo VoteView và PutState vào ledger
6. Trả VoteView dưới dạng JSON string
```

> **Lưu ý:** Không cập nhật counter chung sau bước 5. Mỗi SubmitVote chỉ ghi đúng 1 key riêng (`vote\nelectionID\nvoteID`) nên các phiếu đồng thời không bao giờ đụng key nhau.

**Response:** `VoteView` JSON

```json
{
  "docType": "vote",
  "electionId": "...",
  "voteId": "...",
  "blindedCommitment": "abc123...",
  "txTimestamp": "1700000000",
  "txId": "fabric-transaction-id"
}
```

**Điều kiện từ chối:**
- `electionID` hoặc `voteID` rỗng
- `blindedCommitment` không phải hex 64 chars hợp lệ
- Election đã commit Merkle root (đã đóng)
- `voteID` đã tồn tại trên ledger

---

### 3.2 GetVote

**Loại:** `query` (read-only)

**Mục đích:** Truy vấn bản ghi phiếu bầu từ ledger. Dùng khi verify phiếu sau khi nộp.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |
| `voteID` | string | ID phiếu cần truy vấn |

**Logic thực hiện:**

```
1. Tính composite key: vote\nelectionID\nvoteID
2. GetState → decode JSON → trả VoteView JSON
3. Nếu không tồn tại: trả lỗi
```

**Response:** `VoteView` JSON (cùng format với SubmitVote)

---

## 4. Hàm Merkle Contract

### 4.1 CommitMerkleRoot

**Loại:** `invoke` (write transaction)

**Mục đích:** Commit Merkle root của tất cả phiếu bầu lên ledger khi admin đóng election. Đây là điểm chốt không thể thay đổi — mọi verify sau đó đều so khớp với root này.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |
| `merkleRoot` | string | SHA256 hex 64 chars của Merkle root |
| `voteCountStr` | string | Số phiếu tính theo string (uint64) |

**Logic thực hiện:**

```
1. Validate: electionID không rỗng
2. Parse merkleRoot thành [32]byte (phải là valid hex 64 chars)
3. Parse voteCountStr thành uint64
4. countVotes(electionID): đếm trực tiếp số record "vote" trên ledger bằng
   GetStateByPartialCompositeKey(vote, [electionID])
   → Nếu ledgerCount != voteCount: từ chối (off-chain và on-chain mất đồng bộ)
   → Range read này cũng tạo ràng buộc MVCC: nếu có SubmitVote commit xen vào,
     CommitMerkleRoot sẽ bị invalid và phải chạy lại
5. Kiểm tra root chưa tồn tại (composite key root\nelectionID)
   → Nếu đã tồn tại: từ chối (idempotent, chặn overwrite)
6. Tạo MerkleRootView { committed: true, merkleRoot, voteCount, closedAt, txId }
7. PutState vào ledger
8. Trả MerkleRootView JSON
```

**Response:** `MerkleRootView` JSON

```json
{
  "docType": "root",
  "electionId": "...",
  "merkleRoot": "deadbeef...",
  "voteCount": 42,
  "committed": true,
  "closedAt": "1700000000",
  "txId": "fabric-transaction-id"
}
```

**Điều kiện từ chối:**
- `merkleRoot` không phải hex 64 chars hợp lệ
- `voteCount` không phải số nguyên dương
- `voteCount` không khớp số record `vote` trên ledger
- Merkle root cho election này đã tồn tại

---

### 4.2 GetMerkleRoot

**Loại:** `query` (read-only)

**Mục đích:** Lấy Merkle root đã commit. Dùng để kiểm chứng root từ backend khớp với chain.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |

**Logic thực hiện:**

```
1. loadMerkleRoot(electionID)
2. Nếu root.Committed = false: trả lỗi (root chưa được commit)
3. Trả MerkleRootView JSON
```

**Response:** `MerkleRootView` JSON

---

### 4.3 VerifyVoteReceipt

**Loại:** `query` (read-only)

**Mục đích:** Xác minh một phiếu bầu có thuộc Merkle tree của election không, dựa trên `blindedCommitment` và Merkle proof path. Là bước 7 trong luồng verify phiếu.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |
| `blindedCommitment` | string | SHA256 hex 64 chars của phiếu cần kiểm tra |
| `proofJSON` | string | JSON array string của Merkle proof path |

**Logic thực hiện:**

```
1. Validate blindedCommitment là hex 64 chars hợp lệ
2. leaf = SHA256(UTF8(blindedCommitment))
   NOTE: Hash leaf bằng SHA256 của chuỗi hex (không phải bytes),
         nhất quán với off-chain (libs/fabric/src/lib/merketree/index.ts)
3. loadMerkleRoot(electionID)
4. Nếu root.Committed = false: trả { rootCommitted: false, inElection: false }
5. Parse proofJSON thành []ProofStep { data: [32]byte, position: "left"|"right" }
6. applyProof(leaf, proof) → tính computedRoot từ leaf + proof path
7. So sánh computedRoot với root.MerkleRoot
8. Trả ReceiptVerifyView { inElection: bool, merkleRoot, voteCountOnChain, ... }
```

**Response:** `ReceiptVerifyView` JSON

```json
{
  "electionId": "...",
  "rootCommitted": true,
  "inElection": true,
  "merkleRoot": "deadbeef...",
  "voteCountOnChain": 42
}
```

**Trường hợp `inElection: false`:**
- Proof sai hoặc commitment không thuộc Merkle tree
- Commitment bị giả mạo

---

### 4.4 GetAuditCounts

**Loại:** `query` (read-only)

**Mục đích:** Lấy thống kê audit — số phiếu submit, số phiếu reveal, trạng thái Merkle root. Dùng để so sánh DB vs Chain.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |

**Logic thực hiện:**

```
1. countVotes(electionID): đếm record vote bằng GetStateByPartialCompositeKey
2. aggregateReveals(electionID): duyệt record usedReveal → revealCount (số lá phiếu)
3. loadMerkleRoot(electionID) → { Committed }
4. Trả AuditCountsView
```

**Response:** `AuditCountsView` JSON

```json
{
  "electionId": "...",
  "totalVoteCount": 100,
  "revealCount": 95,
  "rootCommitted": true,
  "revealVoteMatch": false
}
```

- `revealVoteMatch = (totalVoteCount == revealCount)`: tất cả phiếu đã được reveal chưa
- Cả hai con số đều phản ánh trực tiếp số record trên ledger, không qua counter trung gian.

---

## 5. Hàm Reveal Contract

### 5.1 RevealVoteCompact

**Loại:** `invoke` (write transaction)

**Mục đích:** Ghi nhận việc voter giải mù phiếu. Đây là hàm cốt lõi của pha kiểm phiếu. Mỗi lá phiếu có thể chọn **nhiều** ứng viên (`candidateIds`).

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |
| `candidateIdsJson` | string | Chuỗi JSON canonical của mảng ứng viên đã chọn, vd `["64f...a1","64f...c3"]` (đã dedupe + sort lexicographic ở backend) |
| `revealKey` | string | SHA256(h_bytes \|\| sPrime_bytes) hex 64 chars |
| `revealPayloadHash` | string | SHA256("reveal-v2" \|\| uint32be(len(candidateIdsJson)) \|\| candidateIdsJson \|\| h32 \|\| sPrime32) hex 64 chars |

**Logic thực hiện:**

```
1. Validate: electionID không rỗng; parse candidateIdsJson thành []string
   (mảng không rỗng, từng phần tử không rỗng)
2. Parse revealKey thành keyHash [32]byte
3. Parse revealPayloadHash thành payloadHash [32]byte
4. loadMerkleRoot(electionID)
   → Nếu root.Committed = false: từ chối
   (election phải đã đóng mới được reveal)
5. Tính usedKey = composite(usedReveal, electionID, base64URL(keyHash))
6. GetState(usedKey) → kiểm tra revealKey chưa được dùng
   → Nếu đã tồn tại: từ chối (chặn replay attack on-chain)
7. PutState(usedKey, encode(candidateIdsJson, payloadHash))
   (đánh dấu revealKey đã dùng, lưu kèm chuỗi candidateIds và payloadHash)
8. Trả UsedRevealView JSON
```

> **Lưu ý:** Mỗi RevealVoteCompact chỉ ghi đúng 1 key `usedReveal` (key riêng per ballot). Không có counter tally hay RevealCount dùng chung — tránh MVCC_READ_CONFLICT khi nhiều reveal đồng thời. Tally và RevealCount được tổng hợp on-demand bởi `GetTally` / `GetAuditCounts`.

**Response:** `UsedRevealView` JSON

```json
{
  "electionId": "...",
  "candidateIds": ["64f...a1", "64f...c3"],
  "revealKey": "base64url-encoded-key",
  "revealKeyHex": "hex-encoded-key",
  "revealPayloadHash": "base64url-encoded-hash",
  "revealPayloadHashHex": "hex-encoded-hash"
}
```

**Điều kiện từ chối:**
- `candidateIdsJson` không phải JSON array hợp lệ / mảng rỗng / có phần tử rỗng
- `revealKey` hoặc `revealPayloadHash` không phải hex 64 chars
- Election chưa commit Merkle root (chưa đóng)
- `revealKey` đã được dùng (replay attack)

**Lưu ý bảo mật:**
- `revealPayloadHash` lưu nhưng không verify on-chain — chaincode tin tưởng backend đã verify chữ ký EC-Schnorr. Hash này là commitment của `(candidateIds, h, sPrime)` để audit sau nếu cần.
- Chaincode không enforce `maxSelectableCandidates` hay danh sách ứng viên hợp lệ — việc đó do `reveal-vote` service kiểm tra trước khi gọi chaincode.
- Chaincode không verify chữ ký Schnorr — việc này được thực hiện bởi `reveal-vote` service trước khi gọi chaincode.
- Quy ước canonical (dedupe + sort + `JSON.stringify`) của `candidateIds` phải giống hệt giữa client (lúc ký) và backend (lúc verify/hash) để chữ ký verify đúng.

---

### 5.2 GetTally

**Loại:** `query` (read-only)

**Mục đích:** Lấy kết quả kiểm phiếu theo từng ứng viên, tổng hợp trực tiếp từ record `usedReveal`.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |

**Logic thực hiện:**

```
1. aggregateReveals(electionID):
   - GetStateByPartialCompositeKey(usedReveal, [electionID])
   - Với mỗi record: decode candidateIdsJson → parse []string
   - Cộng 1 vào tally[candidateID] cho từng ứng viên trong lá phiếu
2. Trả TallyView { electionId, tally: { candidateId: count } }
```

**Response:** `TallyView` JSON

```json
{
  "electionId": "...",
  "tally": {
    "candidate-id-1": 45,
    "candidate-id-2": 50,
    "candidate-id-3": 5
  }
}
```

> **Lưu ý:** Giá trị của mỗi ứng viên là **số lượt chọn** (selections). Khi bầu nhiều ứng viên, tổng các giá trị trong `tally` có thể lớn hơn số lá phiếu — vì một lá phiếu chọn N ứng viên sẽ cộng N vào tổng tally nhưng chỉ là 1 lá phiếu. Số lá phiếu đã reveal lấy từ `GetAuditCounts.revealCount`.

---

### 5.3 GetUsedReveal

**Loại:** `query` (read-only)

**Mục đích:** Kiểm tra một `revealKey` đã được dùng chưa và gắn với (các) ứng viên nào. Dùng cho audit và recovery.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `electionID` | string | ID của cuộc bầu cử |
| `revealKey` | string | SHA256(h \|\| sPrime) hex 64 chars |

**Logic thực hiện:**

```
1. Parse revealKey → keyHash
2. compositeKey(usedReveal, electionID, base64URL(keyHash))
3. GetState → decode (candidateIdsJson, payloadHash) → parse candidateIds []string
4. Trả UsedRevealView (candidateIds là mảng)
```

---

### 5.4 ComputeRevealPayloadHash

**Loại:** `query` (read-only)

**Mục đích:** Tính `revealPayloadHash` từ `(candidateIds, h, sPrime)`. Hàm utility cho audit và debug — cho phép kiểm chứng hash được ghi trên chain có khớp với input không.

**Tham số:**

| Tham số | Kiểu | Mô tả |
|---|---|---|
| `candidateIdsJson` | string | Chuỗi JSON canonical của mảng ứng viên, vd `["64f...a1","64f...c3"]` |
| `h` | string | Scalar h hex 64 chars |
| `sPrime` | string | Scalar sPrime hex 64 chars |

**Logic thực hiện:**

```
hash = SHA256("reveal-v2" || uint32_big_endian(len(candidateIdsJson)) || candidateIdsJson || h[32] || sPrime[32])
```

**Response:** `PayloadHashView` JSON

```json
{
  "candidateIds": ["64f...a1", "64f...c3"],
  "revealPayloadHash": "base64url...",
  "revealPayloadHashHex": "hex64chars...",
  "hashDefinition": "sha256('reveal-v2' || uint32be(len(candidateIdsJson)) || candidateIdsJson || h32 || sPrime32)"
}
```

---

## 6. Key schema trên ledger

Tất cả key dùng `CreateCompositeKey` của Fabric (dùng ký tự `\x00` làm separator):

```
vote record:   vote      + \x00 + electionId + \x00 + voteId
merkle root:   root      + \x00 + electionId
used reveal:   usedReveal + \x00 + electionId + \x00 + base64URL(revealKeyHash)
```

Không có key `stats` hay `tally` — counter dùng chung đã bị loại bỏ.

### Encoding của value

| State | Encoding |
|---|---|
| `VoteView` | JSON marshal → bytes |
| `MerkleRootView` | JSON marshal → bytes |
| `usedReveal` | binary: `candidateIdsJson_len(uvarint) + candidateIdsJson + payloadHash(32 bytes)` |

---

## 7. Luồng trạng thái election trên chain

Chaincode không lưu trạng thái election, nhưng logic hàm ngầm định trình tự:

```
Phase 1: ACTIVE (nhận phiếu)
  SubmitVote có thể gọi song song nhiều lần
  → Mỗi lần: PutState 1 key vote riêng biệt (không đụng key chung)

Phase 2: CLOSED (đóng bầu cử)
  CommitMerkleRoot gọi đúng 1 lần
  → countVotes() đếm số record "vote" trực tiếp trên ledger
  → Kiểm tra count khớp voteCount arg
  → root.Committed = true
  → Sau bước này: SubmitVote từ chối (election đã đóng)

Phase 3: REVEAL (kiểm phiếu)
  RevealVoteCompact gọi song song nhiều lần
  → Mỗi lần: PutState 1 key usedReveal riêng biệt (không đụng key chung)
  → Điều kiện tiên quyết: root.Committed = true
  → GetTally / GetAuditCounts tổng hợp on-demand từ record usedReveal
```

```
ACTIVE ─────────── nhiều SubmitVote song song ────────────────────────────┐
                   (mỗi tx chỉ ghi 1 key riêng)                           │
                ┌──────────── CommitMerkleRoot ─────────────────────────────▼
                │    (đếm record "vote" trực tiếp, không đọc counter)
                ▼
CLOSED  ──────── nhiều RevealVoteCompact song song ───────────────────────┐
                 (mỗi tx chỉ ghi 1 key riêng)                             │
                                                                           ▼
                                                   GetTally / GetAuditCounts
                                                   (tổng hợp on-demand từ ledger)
```

---

## 8. Kiểu dữ liệu trả về (Response types)

Tất cả hàm trả về JSON string (Go `mustJSON(v)`). Các kiểu được định nghĩa trong `types.go`:

```go
// Phiếu bầu mù
VoteView {
    DocType:            string // "vote"
    ElectionID:         string
    VoteID:             string
    BlindedCommitment:  string // hex 64 chars
    TxTimestampSeconds: string // unix timestamp
    TxID:               string // Fabric transaction ID
}

// Merkle root đã commit
MerkleRootView {
    DocType:           string // "root"
    ElectionID:        string
    MerkleRoot:        string // hex 64 chars
    VoteCount:         uint64
    Committed:         bool
    ClosedAtTxSeconds: string // unix timestamp
    CommitTxID:        string
}

// Kết quả kiểm phiếu theo ứng viên
TallyView {
    ElectionID: string
    Tally:      map[string]uint64  // { candidateId: số lượt chọn }
}

// Thống kê audit (đếm on-demand từ record trên ledger)
AuditCountsView {
    ElectionID:      string
    TotalVoteCount:  uint64 // số record "vote" trên ledger
    RevealCount:     uint64 // số record "usedReveal" trên ledger
    RootCommitted:   bool
    RevealVoteMatch: bool   // TotalVoteCount == RevealCount
}

// Bản ghi reveal đã dùng
UsedRevealView {
    ElectionID:         string
    CandidateIDs:       []string // danh sách ứng viên đã chọn (canonical)
    RevealKey:          string   // base64URL encoded
    RevealKeyHex:       string   // hex 64 chars
    RevealPayloadHash:  string   // base64URL encoded
    RevealPayloadHashH: string   // hex 64 chars
}

// Kết quả xác minh receipt
ReceiptVerifyView {
    ElectionID:       string
    RootCommitted:    bool
    InElection:       bool   // true nếu commitment thuộc Merkle tree
    MerkleRoot:       string // hex 64 chars (nếu committed)
    ProofError:       string // error khi parse proof (nếu có)
    VoteCountOnChain: uint64
}

// Hash payload reveal
PayloadHashView {
    CandidateIDs:         []string // danh sách ứng viên (canonical)
    RevealPayloadHash:    string   // base64URL
    RevealPayloadHashHex: string   // hex 64 chars
    HashDefinition:       string   // mô tả công thức hash
}
```

---

## 9. Thiết kế tránh MVCC conflict

### Vấn đề (trước khi sửa)

Hyperledger Fabric dùng MVCC (Multi-Version Concurrency Control): trong một block, tất cả transaction được simulate song song trên cùng snapshot của World State, sau đó committer validate tuần tự. Tx nào đọc một key ở version X mà key đó đã được tx trước trong cùng block ghi → **MVCC_READ_CONFLICT → invalid** — toàn bộ write của tx đó bị rollback, kể cả vote record.

Chaincode cũ dùng **counter dùng chung** (`stats.TotalVoteCount`, `stats.RevealCount`, `tally[candidateId]`) — tất cả SubmitVote trong cùng một block đều đọc-sửa-ghi cùng key `stats`, dẫn đến chỉ tx đầu tiên valid, các tx còn lại bị invalid. Kết quả: `totalVoteCount ≈ số block`, không phải số phiếu.

### Giải pháp (hiện tại)

| Hàm | Key ghi | Concurrent safe? |
|---|---|---|
| `SubmitVote` | `vote\nelectionId\nvoteId` (riêng per vote) | Có — key khác nhau |
| `RevealVoteCompact` | `usedReveal\nelectionId\nbase64(keyHash)` (riêng per reveal) | Có — key khác nhau |
| `CommitMerkleRoot` | `root\nelectionId` (chỉ ghi 1 lần) | Có — idempotent |

Các hàm **query tổng hợp** (`GetTally`, `GetAuditCounts`, `CommitMerkleRoot`) dùng `GetStateByPartialCompositeKey` để đếm/tổng hợp on-demand:

- `countVotes`: range scan prefix `vote\nelectionId` → đếm số record.
- `aggregateReveals`: range scan prefix `usedReveal\nelectionId` → decode `candidateIdsJson` từng record → cộng dồn vào tally map và revealCount.

Range read trong `CommitMerkleRoot` tạo ràng buộc MVCC có chủ đích: nếu có `SubmitVote` commit xen vào đúng lúc chốt sổ, `CommitMerkleRoot` sẽ bị invalid và phải gọi lại — đảm bảo `voteCount` luôn chính xác tại thời điểm commit.
