# Đề Bài Tuyển Dụng Lập Trình Viên Golang - Chủ Đề Blockchain


## Yêu Cầu Chung

Xây dựng một hệ thống blockchain đơn giản mô phỏng việc chuyển tiền giữa 2 người dùng: **Alice** và **Bob**. Hệ thống cần đảm bảo các tính chất sau:

### 1. Ký số giao dịch bằng ECDH

Việc đảm bảo tính toàn vẹn và không thể chối bỏ của giao dịch là cốt lõi trong blockchain.

  * **Mỗi người dùng (Alice, Bob)** sẽ có cặp khóa riêng (private/public) sử dụng thuật toán **ECDSA** (Elliptic Curve Digital Signature Algorithm). **Lưu ý quan trọng**: Đề bài ban đầu ghi là ECDH, nhưng để ký số giao dịch, **ECDSA** mới là thuật toán phù hợp. ECDH dùng để trao đổi khóa an toàn. Hãy đảm bảo bạn sử dụng **ECDSA** cho việc ký và xác thực chữ ký giao dịch.
  * Một **giao dịch (transaction)** bao gồm:
      * `Sender`: Địa chỉ công khai của người gửi.
      * `Receiver`: Địa chỉ công khai của người nhận.
      * `Amount`: Số lượng tiền được chuyển.
      * `Timestamp`: Thời điểm tạo giao dịch.
      * `Signature`: Chữ ký điện tử của giao dịch, được tạo bởi **private key** của người gửi.
  * Hệ thống cần **xác thực chữ ký** khi nhận giao dịch để đảm bảo người gửi thực sự là chủ sở hữu số tiền đó và giao dịch không bị thay đổi.

### 2. Lưu trữ dữ liệu trên LevelDB, xác thực bằng Merkle Tree

Mỗi node validator cần duy trì một bản sao của blockchain một cách nhất quán và đáng tin cậy.

  * Mỗi node validator sẽ lưu các **block** trong **LevelDB**. LevelDB là một cơ sở dữ liệu key-value cục bộ, đơn giản và hiệu quả.
  * Mỗi **block** bao gồm:
      * `Danh sách giao dịch`: Tập hợp các giao dịch hợp lệ được đưa vào block này.
      * `Root của Merkle Tree`: Hash gốc của cây Merkle được xây dựng từ danh sách giao dịch.
      * `Hash của block trước (PreviousBlockHash)`: Liên kết block hiện tại với block trước đó, tạo thành chuỗi.
      * `Hash của block hiện tại (CurrentBlockHash)`: Hash của toàn bộ block (bao gồm tất cả các trường trên).
  * **Triển khai Merkle Tree**: Đây là một phần quan trọng để xác minh tính toàn vẹn của giao dịch trong block một cách hiệu quả. Đảm bảo rằng **hash của Merkle Tree phản ánh đúng nội dung của tất cả các giao dịch trong block** và bất kỳ thay đổi nhỏ nào trong giao dịch cũng sẽ làm thay đổi Merkle Root.

### 3. Cơ chế đồng thuận giữa 3 node validator (chạy Docker)

Blockchain cần một cơ chế để các node trong mạng lưới đồng ý về trạng thái chung của sổ cái.

  * **Khởi tạo 3 node validator** chạy trên Docker. Mỗi node sẽ là một dịch vụ độc lập.
  * **Cơ chế Leader-Follower**: Một node sẽ đóng vai trò là **Leader** (có thể được bầu chọn ngẫu nhiên, hoặc là node đầu tiên được khởi động, hoặc cấu hình tĩnh). Leader chịu trách nhiệm:
      * Tạo các block mới từ các giao dịch đang chờ xử lý.
      * Gửi block đề xuất (proposed block) đến các node còn lại (Followers).
  * **Cơ chế bỏ phiếu (Voting)**: Các Followers sau khi nhận được block đề xuất từ Leader sẽ:
      * Xác thực tính hợp lệ của block (kiểm tra chữ ký giao dịch, Merkle Root, PreviousBlockHash, v.v.).
      * Gửi lại phiếu bầu (vote) cho Leader (chấp nhận hoặc từ chối).
  * **Đồng thuận (Consensus)**: Sau khi đạt đủ số lượng phiếu (ví dụ: **majority vote**, tức là 2/3 tổng số node, nghĩa là 2/3 trong 3 node, tức 2 phiếu đồng thuận), block được xem là đã được đồng thuận.
  * **Commit Block**: Sau khi đồng thuận, Leader thông báo cho tất cả các node để **commit** (lưu) block vào LevelDB cục bộ của họ.

### 4. Cơ chế khôi phục node validator khi bị ngắt kết nối

Hệ thống phải có khả năng chống chịu lỗi và tự phục hồi.

  * Nếu một node validator bị tắt/mất kết nối (ví dụ: bị dừng container Docker):
      * Khi được khởi động lại, node phải tự động **kết nối lại** với các node khác trong mạng (ví dụ: thông qua một danh sách node peer được cấu hình sẵn hoặc cơ chế khám phá dịch vụ).
      * Node cần **tải các block đã bỏ lỡ** từ các node còn lại. Điều này có thể thực hiện thông qua một API/gRPC endpoint cho phép node yêu cầu các block từ một `blockHeight` cụ thể trở đi.
      * Sau khi **đồng bộ hóa** hoàn toàn (bắt kịp với chuỗi block dài nhất và hợp lệ nhất), node tiếp tục tham gia vào quá trình đồng thuận và xử lý giao dịch.

-----

## Yêu Cầu Bài Nộp

Để đánh giá đầy đủ, bạn cần cung cấp:

  * **Source code Golang đầy đủ**: Mã nguồn phải sạch sẽ, dễ đọc, có cấu trúc tốt và tuân thủ các nguyên tắc lập trình Golang.
  * **File `docker-compose.yml`**: Để khởi chạy 3 validator một cách dễ dàng và nhanh chóng. File này nên bao gồm cấu hình mạng, cổng, và biến môi trường cần thiết cho mỗi node.
  * **Script CLI hoặc REST API**:
      * Tạo user (Alice, Bob): Cho phép tạo cặp khóa ECDSA và lưu trữ chúng an toàn (ví dụ: trong một file JSON đơn giản hoặc một cơ sở dữ liệu nhỏ).
      * Gửi giao dịch từ Alice đến Bob: Cho phép người dùng tạo và gửi giao dịch (ký bởi Alice) đến một trong các node validator.
      * Kiểm tra trạng thái node / block / đồng thuận: API hoặc CLI để kiểm tra chiều dài chuỗi block hiện tại, xem nội dung của một block cụ thể, hoặc trạng thái của các node trong quá trình đồng thuận.
  * **Tài liệu hướng dẫn chạy hệ thống và mô tả kiến trúc**: Một file `README.md` chi tiết hướng dẫn cách setup, chạy, và tương tác với hệ thống. Nêu rõ các quyết định kiến trúc chính, các thư viện sử dụng và lý do lựa chọn.

-----

## Gợi Ý / Hướng Dẫn Chi Tiết

Để giúp bạn bắt đầu và hoàn thành bài toán một cách hiệu quả:

### 1. Cấu trúc dự án Golang

  * **Tổ chức code theo package**:
      * `pkg/blockchain`: Chứa định nghĩa `Block`, `Transaction`, logic tạo hash, Merkle Tree.
      * `pkg/wallet`: Chứa logic tạo và quản lý cặp khóa ECDSA, ký và xác thực chữ ký.
      * `pkg/p2p`: Xử lý giao tiếp giữa các node (gRPC hoặc HTTP), bao gồm phát tán giao dịch, đề xuất/bỏ phiếu block.
      * `pkg/consensus`: Triển khai cơ chế đồng thuận (ví dụ: Leader bầu chọn, bỏ phiếu).
      * `pkg/storage`: Tương tác với LevelDB.
      * `cmd/cli`: Xử lý các lệnh CLI.
      * `cmd/node`: Điểm khởi chạy của mỗi node validator.
  * **Sử dụng Go Modules**: Đảm bảo quản lý dependencies hiệu quả.

### 2. Triển khai ECDSA

  * **Tạo cặp khóa**: Sử dụng `crypto/elliptic` (ví dụ: `elliptic.P256()`) và `crypto/ecdsa`.
    ```go
    import (
        "crypto/ecdsa"
        "crypto/elliptic"
        "crypto/rand"
        "fmt"
    )

    func GenerateKeyPair() (*ecdsa.PrivateKey, error) {
        privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
        if err != nil {
            return nil, fmt.Errorf("failed to generate key pair: %w", err)
        }
        return privKey, nil
    }

    func PublicKeyToAddress(pubKey *ecdsa.PublicKey) []byte {
        // Chuyển public key thành dạng địa chỉ (ví dụ: hash của public key)
        // Đảm bảo địa chỉ là duy nhất và cố định
        return nil // Implement this
    }
    ```
  * **Ký giao dịch**: Sử dụng `ecdsa.Sign()`. Bạn cần hash nội dung giao dịch (sender, receiver, amount, timestamp) trước khi ký.
    ```go
    import (
        "crypto/sha256"
        "encoding/json"
        "fmt"
        "math/big"
    )

    type Transaction struct {
        Sender    []byte // Public Key or Address
        Receiver  []byte // Public Key or Address
        Amount    float64
        Timestamp int64
        Signature []byte // R and S concatenated
    }

    func (t *Transaction) Hash() []byte {
        // Create a hashable representation of the transaction
        txCopy := *t
        txCopy.Signature = nil // Exclude signature from hash
        data, _ := json.Marshal(txCopy)
        hash := sha256.Sum256(data)
        return hash[:]
    }

    func SignTransaction(tx *Transaction, privKey *ecdsa.PrivateKey) error {
        txHash := tx.Hash()
        r, s, err := ecdsa.Sign(rand.Reader, privKey, txHash)
        if err != nil {
            return fmt.Errorf("failed to sign transaction: %w", err)
        }
        // Store R and S as a concatenated byte slice
        tx.Signature = append(r.Bytes(), s.Bytes()...)
        return nil
    }
    ```
  * **Xác thực chữ ký**: Sử dụng `ecdsa.Verify()`. Cần tách R và S từ chữ ký.
    ```go
    func VerifyTransaction(tx *Transaction, pubKey *ecdsa.PublicKey) bool {
        txHash := tx.Hash()
        // Assume signature is r and s concatenated, parse them back to big.Int
        r := new(big.Int).SetBytes(tx.Signature[:len(tx.Signature)/2])
        s := new(big.Int).SetBytes(tx.Signature[len(tx.Signature)/2:])
        return ecdsa.Verify(pubKey, txHash, r, s)
    }
    ```

### 3. Merkle Tree

  * **Input**: Danh sách các hash của từng giao dịch trong block.
  * **Quá trình xây dựng**: Ghép cặp các hash, hash chúng lại, và lặp lại cho đến khi còn một hash duy nhất (Merkle Root). Nếu số lượng hash lẻ, copy hash cuối cùng.
  * Thư viện: Có thể tự triển khai hoặc tìm các thư viện Golang mã nguồn mở cho Merkle Tree (ví dụ: `github.com/cbergoon/merkletree`). Tuy nhiên, việc tự triển khai sẽ thể hiện được sự hiểu biết sâu sắc của bạn.

### 4. LevelDB

  * **Key-value store**: LevelDB lưu trữ dữ liệu dưới dạng cặp key-value bytes.
  * **Lưu Block**: Key có thể là `block_hash` hoặc `block_height`. Value là byte representation của block.
    ```go
    import (
        "encoding/json"
        "github.com/syndtr/goleveldb/leveldb"
    )

    type Block struct {
        // ... fields ...
    }

    func SaveBlock(db *leveldb.DB, block *Block) error {
        blockBytes, err := json.Marshal(block)
        if err != nil {
            return err
        }
        return db.Put([]byte(block.Hash()), blockBytes, nil)
    }

    func GetBlock(db *leveldb.DB, hash []byte) (*Block, error) {
        blockBytes, err := db.Get(hash, nil)
        if err != nil {
            return nil, err
        }
        var block Block
        if err := json.Unmarshal(blockBytes, &block); err != nil {
            return nil, err
        }
        return &block, nil
    }
    ```
  * **Xử lý lỗi**: Luôn kiểm tra lỗi khi tương tác với LevelDB.

### 5. Giao tiếp giữa các Node (P2P)

  * **gRPC**: Đây là lựa chọn tuyệt vời cho giao tiếp hiệu năng cao giữa các dịch vụ trong Docker. Bạn sẽ định nghĩa các `.proto` file cho các thông điệp và service như:
      * `SendTransaction(Transaction)`
      * `ProposeBlock(Block)`
      * `Vote(Vote)`
      * `GetBlock(BlockHeight)`
      * `GetLatestBlock()`
  * **HTTP/REST API**: Đơn giản hơn để bắt đầu nhưng có thể kém hiệu quả hơn gRPC cho các tác vụ đồng bộ hóa khối lượng lớn.
  * **Kết nối**: Mỗi node cần biết địa chỉ của các node peer khác (có thể cấu hình trong `docker-compose.yml` hoặc biến môi trường).

### 6. Cơ chế đồng thuận đơn giản (Majority Vote)

  * **Leader election**: Có thể đơn giản bằng cách cấu hình cứng (ví dụ: node 1 là leader) hoặc sử dụng một cơ chế bầu chọn đơn giản khi khởi động.
  * **State Machine**: Mỗi node validator sẽ có một trạng thái (ví dụ: `Leader`, `Follower`, `Syncing`).
  * **Quản lý giao dịch pending**: Leader thu thập các giao dịch chưa được xác nhận (pending transactions).
  * **Thời gian định kỳ**: Leader có thể tạo block mới theo một khoảng thời gian cố định (ví dụ: mỗi 5 giây) hoặc khi có đủ số lượng giao dịch.
  * **Xử lý vote**: Leader sẽ nhận phiếu bầu từ Followers. Sau khi nhận đủ số phiếu chấp thuận, block được xem là finalized.
  * **Block propagation**: Khi block được finalized, Leader sẽ thông báo cho tất cả các node để lưu block đó.

### 7. Khôi phục Node

  * **Polling/Periodic check**: Node bị ngắt kết nối khi khởi động lại có thể định kỳ kiểm tra trạng thái của các node khác trong mạng.
  * **Get latest block**: Khi kết nối lại, node sẽ gọi API `GetLatestBlock()` từ các node khác để biết `blockHeight` hiện tại của mạng.
  * **Syncing**: Sau đó, node sẽ lặp lại việc gọi `GetBlock(height)` cho từng block mà nó bị thiếu cho đến khi đạt được `blockHeight` mới nhất.
  * **Đảm bảo tính nhất quán**: Khi đồng bộ, cần xác minh chuỗi block (hash trước đó phải khớp, Merkle Root phải hợp lệ) để tránh các chuỗi giả mạo.

-----

## Docker Setup Gợi ý

Bạn có thể bắt đầu với một `docker-compose.yml` đơn giản như sau:

```yaml
version: '3.8'

services:
  node1:
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      NODE_ID: node1
      # Cấu hình các peer khác. Có thể truyền dưới dạng biến môi trường hoặc file config
      PEERS: node2:50051,node3:50051
      # Nếu dùng Leader tĩnh
      IS_LEADER: "true" # Chỉ node1 là leader ban đầu
    ports:
      - "50051:50051" # Cổng gRPC/HTTP
      - "8080:8080" # Cổng API/CLI nếu có
    volumes:
      - node1_data:/app/data # Để LevelDB lưu dữ liệu liên tục

  node2:
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      NODE_ID: node2
      PEERS: node1:50051,node3:50051
      IS_LEADER: "false"
    ports:
      - "50052:50051"
    volumes:
      - node2_data:/app/data

  node3:
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      NODE_ID: node3
      PEERS: node1:50051,node2:50051
      IS_LEADER: "false"
    ports:
      - "50053:50051"
    volumes:
      - node3_data:/app/data

volumes:
  node1_data:
  node2_data:
  node3_data:
```

Và một `Dockerfile` cơ bản:

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /go-blockchain ./cmd/node

# Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /go-blockchain .
# Tạo thư mục data cho LevelDB
RUN mkdir -p /app/data
CMD ["./go-blockchain"]
```

