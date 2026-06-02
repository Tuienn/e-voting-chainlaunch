# BÁO CÁO NGHIÊN CỨU VÀ TRIỂN KHAI HỆ THỐNG MẠNG BLOCKCHAIN HYPERLEDGER FABRIC PHỤC VỤ ỨNG DỤNG E-VOTING

---

## 1: THIẾT KẾ KIẾN TRÚC MẠNG VÀ CHUẨN BỊ MÔI TRƯỜNG

### 1.1 Sơ đồ cấu trúc mạng (Network Topology)

Hệ thống bầu cử điện tử (E-voting) yêu cầu mức độ sẵn sàng cao và khả năng chống lỗi tốt. Dựa trên mô hình tài liệu kỹ thuật, mạng blockchain được xây dựng trên nền tảng Hyperledger Fabric, cấu hình định danh tổ chức thuộc `org1msp` với tên kênh truyền thông là `votechannel`.

Hệ thống hạ tầng phân rã bao gồm 5 nút cốt lõi

- **Thành phần đồng thuận (Ordering Service):** Gồm 03 nút cấu hình theo thuật toán Raft (`orderer0-org1msp`, `orderer1-org1msp`, `orderer2-org1msp`) chạy lần lượt trên các cổng 9000, 9100 và 9200 để đảm bảo tính toàn vẹn dữ liệu và phân tách khối giao dịch.
- **Thành phần lưu trữ và xác thực (Peer Nodes):** Gồm 02 nút Peer (`peer0-org1msp` trên cổng 7000 và `peer1-org1msp` trên cổng 7100) đóng vai trò lưu trữ bản sao sổ cái (Ledger) và thực thi hợp đồng thông minh.

### 1.2 Cài đặt công cụ quản trị dòng lệnh (CLI)

Để thực hiện các thao tác cấu hình hệ thống một cách nhất quán và tự động hóa qua kịch bản (scripting), nhóm nghiên cứu sử dụng bộ công cụ giao diện dòng lệnh thay thế cho giao diện đồ họa. Tiến trình cài đặt môi trường thông qua trình quản lý gói:

```bash
npm install -g @chainlaunch/cli

```

---

## CHƯƠNG 2: QUY TRÌNH KHỞI TẠO VÀ CẤU HÌNH HẠ TẦNG

Mọi thao tác khởi tạo tài nguyên mạng được thực hiện tuần tự thông qua việc gọi API của hệ thống điều phối, đảm bảo tính minh bạch và ghi vết toàn bộ vòng đời các nút mạng.

### 2.1 Khởi tạo các nút đồng thuận (Orderer Nodes)

Tiến hành cấu hình và kích hoạt cụm đồng thuận gồm 3 nút độc lập nhằm mục đích loại bỏ điểm lỗi duy nhất (Single Point of Failure):

```bash
# Thiết lập nút Orderer thứ nhất
chainlaunch node create \
  --name "orderer0-org1msp" \
  --type "FABRIC_ORDERER" \
  --mspid "org1msp" \
  --port 9000

# Thiết lập nút Orderer thứ hai
chainlaunch node create \
  --name "orderer1-org1msp" \
  --type "FABRIC_ORDERER" \
  --mspid "org1msp" \
  --port 9100

# Thiết lập nút Orderer thứ ba
chainlaunch node create \
  --name "orderer2-org1msp" \
  --type "FABRIC_ORDERER" \
  --mspid "org1msp" \
  --port 9200

```

### 2.2 Khởi tạo các nút mạng (Peer Nodes)

Khởi tạo cấu trúc hạ tầng cho hai nút xử lý, chịu trách nhiệm nhận các đề xuất giao dịch bầu chọn từ phía người dùng:

```bash
# Thiết lập nút Peer thứ nhất
chainlaunch node create \
  --name "peer0-org1msp" \
  --type "FABRIC_PEER" \
  --mspid "org1msp" \
  --port 7000

# Thiết lập nút Peer thứ hai
chainlaunch node create \
  --name "peer1-org1msp" \
  --type "FABRIC_PEER" \
  --mspid "org1msp" \
  --port 7100

```

---

## 3: THIẾT LẬP KÊNH TRUYỀN THÔNG (CHANNEL MANAGEMENT)

Kênh truyền thông phi tập trung `votechannel` đóng vai trò là một không gian mạng riêng tư, cô lập dữ liệu bầu chọn nhằm bảo mật thông tin giữa các thực thể tham gia.

### 3.1 Khởi tạo mạng phân tán với cụm Orderer

Quá trình tạo lập kênh bắt đầu bằng việc liên kết cấu trúc định tuyến của các nút Orderer để tạo ra một trục xương sống định danh:

```bash
chainlaunch network create \
  --name "votechannel" \
  --platform "fabric" \
  --orderers "orderer0-org1msp,orderer1-org1msp,orderer2-org1msp"

```

### 3.2 Tích hợp các nút Peer vào kênh `votechannel`

Sau khi trục giao thức điều hướng được xác lập, các Peer tiến hành gửi yêu cầu tham gia (Join) để đồng bộ hóa trạng thái cấu hình ban đầu:

```bash
# Tích hợp nút Peer 0 vào hệ thống mạng chung
chainlaunch network join-node \
  --network-id "net-xyz123" \
  --node "peer0-org1msp"

# Tích hợp nút Peer 1 vào hệ thống mạng chung
chainlaunch network join-node \
  --network-id "net-xyz123" \
  --node "peer1-org1msp"

```

---

## 4: TRIỂN KHAI HỢP ĐỒNG THÔNG MINH (SMART CONTRACT LIFECYCLE)

Mã nguồn xử lý logic bầu chọn (vòng đời phiếu bầu, kiểm phiếu tự động) được phát triển bằng ngôn ngữ **Go (Golang)**. Để tối ưu hóa quy trình phân phối và cài đặt, mã nguồn được đóng gói thành một Docker Image và lưu trữ công khai trên Docker Hub với định danh `tuienn/votecc:latest`.

### 4.1 Định nghĩa cấu trúc Chaincode trên mạng lưới

Khai báo siêu dữ liệu (metadata) của hợp đồng thông minh để tạo sự đồng thuận về phiên bản giữa các bên:

```bash
chainlaunch chaincode define \
  --network-id "net-xyz123" \
  --name "votecc" \
  --version "1.0" \
  --sequence 1 \
  --image "docker.io/tuienn/votecc:latest" \
  --init-required true

```

### 4.2 Cài đặt và kích hoạt (Deploy & Commit)

Tiến hành Container Image từ Docker Hub về các nút xác thực nội bộ nhằm thực thi logic nghiệp vụ khi có giao dịch phát sinh:

```bash
chainlaunch chaincode deploy \
  --network-id "net-xyz123" \
  --name "votecc" \
  --peers "peer0-org1msp,peer1-org1msp"

```

### 4.3 Khởi tạo trạng thái sổ cái (Chaincode Initialization)

Thực hiện một giao dịch đặc biệt nhằm thiết lập trạng thái gốc cho sổ cái (ví dụ: tạo danh sách ứng viên ban đầu), áp dụng phương thức kích hoạt có điều kiện:

```bash
chainlaunch chaincode invoke \
  --network-id "net-xyz123" \
  --name "votecc" \
  --fcn "InitLedger" \
  --args "[]"

```

---

## CHƯƠNG 5: QUẢN LÝ ĐỊNH DANH VÀ PHÂN QUYỀN (IDENTITY MANAGEMENT)

Kiến trúc an ninh của Hyperledger Fabric dựa trên nền tảng hạ tầng khóa công khai (PKI). Để ứng dụng phía máy khách (E-voting Client) có quyền tương tác hợp pháp với blockchain, một thực thể định danh mới cần được cấp phát bởi Cơ quan chứng nhận (Certificate Authority - CA).

Quy trình sử dụng tài khoản `admin` cấp cao của CA tổ chức để tạo tài khoản ứng dụng phụ trợ được chia làm 2 giai đoạn:

### 5.1 Đăng ký định danh mới trên hệ thống (Registration Phase)

Khai báo sự tồn tại của tài khoản khách `e-voting-client` vào cơ sở dữ liệu định danh của cấu trúc nội bộ:

```bash
# Thiết lập đường dẫn lưu trữ cấu hình chứng chỉ CA của tổ chức
export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/org1

# Gọi hàm đăng ký tài khoản với vai trò là một client
fabric-ca-client register \
  --url https://localhost:7054 \
  --id.name "e-voting-client" \
  --id.secret "ClientSecret123" \
  --id.type "client" \
  --id.affiliation "org1" \
  --tls.certfiles $FABRIC_CA_CLIENT_HOME/tls-root-cert.pem

```

### 5.2 Phát hành cặp khóa và chứng chỉ mật mã (Enrollment Phase)

Sử dụng mã bí mật vừa được cấp (Token Secret) để yêu cầu máy chủ CA ký phát hành chứng chỉ số X.509 chính thức:

```bash
fabric-ca-client enroll \
  --url https://e-voting-client:ClientSecret123@localhost:7054 \
  --mspdir $FABRIC_CA_CLIENT_HOME/e-voting-client/msp \
  --tls.certfiles $FABRIC_CA_CLIENT_HOME/tls-root-cert.pem

```

### 5.3 Kết quả thực nghiệm và ứng dụng chứng chỉ

Sau khi thực thi thành công các câu lệnh, hệ thống sinh ra cấu trúc thư mục chứng chỉ kỹ thuật tại đường dẫn `$FABRIC_CA_CLIENT_HOME/e-voting-client/msp`:

- `signcerts/cert.pem`: Chứng chỉ xác thực dùng làm định danh công khai cho Client trong mạng.
- `keystore/`: Chứa khóa bí mật (Private Key) dùng để ký số vào các giao dịch bầu chọn (Vote Transactions), ngăn chặn hành vi giả mạo kết quả bầu cử.

Các tệp tin mật mã này sẽ được cấu hình trực tiếp vào cấu thức kết nối (Connection Profile / Wallet API) của mã nguồn ứng dụng client để hoàn thiện giải pháp E-voting toàn diện.
