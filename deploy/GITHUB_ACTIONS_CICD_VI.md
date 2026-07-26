# GitHub Actions CI/CD cho MoneyTracking

Tài liệu này giải thích theo hướng "vừa làm vừa hiểu" để bạn không chỉ chạy được mà còn nắm rõ vì sao hệ thống hoạt động như vậy.

## 1. Mục tiêu của hệ thống này

Project backend của bạn dùng:

- `GitHub Actions` để tự động chạy kiểm tra và deploy
- `GHCR` để lưu Docker image đã build
- `SSH` để GitHub Actions kết nối vào VPS
- `Docker Compose` để VPS kéo image mới và chạy lại container

Mục tiêu là:

- Khi mở `pull request`: chạy CI để kiểm tra code
- Khi push lên `main`: build image, push lên registry, rồi deploy lên VPS
- VPS không cần giữ source code repo để deploy

## 2. GitHub Actions là gì

`GitHub Actions` là hệ thống automation nằm ngay trong GitHub.

Nó cho phép bạn định nghĩa các workflow bằng file YAML trong thư mục:

```text
.github/workflows/
```

Mỗi workflow là một chuỗi job tự động, ví dụ:

- checkout code
- chạy test
- build Docker image
- push image
- SSH vào server để deploy

Trong project này:

- `backend-ci.yml` lo phần kiểm tra chất lượng code
- `backend-cd.yml` lo phần publish image và deploy production

## 3. CI là gì, CD là gì

`CI` là `Continuous Integration`.

Ý nghĩa thực tế:

- mỗi lần bạn push code hoặc mở PR
- hệ thống tự chạy test/check
- nếu lỗi thì biết sớm

Trong repo này, CI đang làm:

- `go test`
- `go test -race`
- `go vet`
- `golangci-lint`
- `gosec`
- `govulncheck`
- validate `docker compose config`
- build Docker image

`CD` là `Continuous Delivery` hoặc `Continuous Deployment`.

Trong repo này, CD đang làm:

- build image production
- push image lên `ghcr.io`
- SSH vào VPS
- chạy `docker compose pull`
- chạy `docker compose up -d --remove-orphans`

Nói ngắn gọn:

- `CI` kiểm tra code có ổn không
- `CD` đưa version mới lên server

## 4. Vì sao dùng GitHub Actions thay vì Jenkins

Với project này, `GitHub Actions` phù hợp hơn `Jenkins` vì:

- repo đang ở GitHub
- không cần chạy thêm một service CI riêng
- đỡ phải tự update plugin, bảo mật, backup Jenkins
- cấu hình nằm ngay trong repo nên dễ theo dõi

`Jenkins` không phải công cụ tệ, nhưng với dự án cá nhân hoặc nhóm nhỏ thì nó nặng vận hành hơn mức cần thiết.

## 5. Luồng deploy thực tế của project này

Luồng đầy đủ hiện tại là:

1. Bạn push code lên GitHub
2. GitHub Actions chạy workflow
3. Workflow build Docker image cho từng service
4. Workflow push các image đó lên `GHCR`
5. Workflow SSH vào VPS bằng SSH key
6. Workflow copy `compose.yaml` và file override production lên VPS
7. VPS đăng nhập `GHCR`
8. VPS kéo image mới về bằng `docker compose pull`
9. VPS chạy lại service bằng `docker compose up -d --remove-orphans`

Điểm quan trọng:

- GitHub Actions là nơi `build`
- VPS là nơi `run`
- `GHCR` là nơi `store image`

Ba phần này tách nhau ra để hệ thống dễ quản lý hơn.

## 6. GHCR là gì và vì sao cần nó

`GHCR` là `GitHub Container Registry`.

Hiểu đơn giản:

- nó là kho chứa Docker image
- giống như Docker Hub, nhưng nằm trong hệ sinh thái GitHub

Vì sao cần GHCR:

- GitHub Actions build image xong phải có chỗ lưu
- VPS cần một nơi để kéo image mới về
- không cần copy source code lên VPS để build trực tiếp trên server

Trong hệ thống này:

- GitHub Actions push image lên `ghcr.io`
- VPS chỉ pull image đó về chạy

## 7. SSH là gì

`SSH` là cách đăng nhập bảo mật vào server từ xa.

Trong dự án này, SSH được dùng để:

- GitHub Actions kết nối vào VPS
- chạy lệnh deploy trên VPS

Bạn có thể hiểu đơn giản:

- bình thường bạn tự gõ `ssh my-vps`
- ở đây GitHub Actions làm việc đó thay bạn

## 8. Public key và private key là gì

SSH key luôn đi theo cặp:

- `private key`
- `public key`

### Private key là gì

`Private key` là chìa khóa bí mật.

Đặc điểm:

- chỉ người sở hữu mới được giữ
- không được commit lên repo
- không được gửi cho người khác
- trong project này, nó sẽ được lưu trong `GitHub Secrets`

Ví dụ file private key:

```text
id_ed25519
```

### Public key là gì

`Public key` là chìa khóa công khai.

Đặc điểm:

- có thể đưa lên server
- không sao nếu người khác nhìn thấy
- nó không cho phép đăng nhập nếu không có private key tương ứng

Ví dụ file public key:

```text
id_ed25519.pub
```

## 9. Vì sao public key đặt ở VPS còn private key đặt ở GitHub

Mục tiêu là để GitHub Actions chứng minh với VPS rằng:

"Tôi đúng là người được phép đăng nhập."

Cách làm:

- VPS giữ `public key` trong `~/.ssh/authorized_keys`
- GitHub Actions giữ `private key` trong `GitHub Secrets`

Khi deploy:

- GitHub Actions dùng private key để ký/xác thực
- VPS dùng public key đã lưu để kiểm tra

Nếu khớp:

- VPS cho phép SSH vào

Nếu không khớp:

- VPS từ chối

## 10. Nếu repo public thì vì sao vẫn an toàn

Repo public vẫn an toàn nếu bạn làm đúng:

- `private key` không nằm trong repo
- password server không nằm trong repo
- `.env` production không nằm trong repo
- JWT private key không nằm trong repo

Repo public có thể chứa:

- workflow YAML
- compose file
- tài liệu hướng dẫn
- thậm chí `public key` cũng không phải thứ bí mật

Điều nguy hiểm là:

- `private key`
- password DB
- password Redis
- password RabbitMQ
- SMTP password
- JWT private key

Các giá trị này phải nằm trong:

- `GitHub Secrets`
- hoặc file trên VPS

## 11. GitHub Secrets là gì

`GitHub Secrets` là nơi GitHub lưu thông tin nhạy cảm để workflow dùng.

Ví dụ:

- SSH private key
- password database
- password Redis
- JWT metadata
- SMTP password

Workflow có thể đọc secret qua cú pháp:

```yaml
${{ secrets.TEN_SECRET }}
```

Secret này:

- không hiện plain text trong repo
- không commit vào git
- chỉ workflow mới dùng được

## 12. Biến nào là secret, biến nào không nhất thiết là secret

### Nên để trong GitHub Secrets

- `DEPLOY_HOST`
- `DEPLOY_PORT`
- `DEPLOY_USER`
- `DEPLOY_PATH`
- `DEPLOY_SSH_KEY`
- `AUTH_DATABASE_URL`
- `GROUP_DATABASE_URL`
- `POSTGRES_DB`
- `POSTGRES_USERNAME`
- `POSTGRES_PASSWORD`
- `GROUP_POSTGRES_DB`
- `GROUP_POSTGRES_USERNAME`
- `GROUP_POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `RABBITMQ_USERNAME`
- `RABBITMQ_PASSWORD`
- `JWT_KEY_ID`
- `JWT_PUBLIC_KEYS`
- `JWT_REFRESH_TTL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `EMAIL_FROM`
- `LOG_LEVEL`
- `CORS_ALLOWED_ORIGIN_REGEX`

Lưu ý:

- Có những biến như `LOG_LEVEL`, `DEPLOY_PORT` hoặc `EMAIL_FROM` về mặt kỹ thuật không quá bí mật
- nhưng để tất cả trong `Secrets` giúp bạn quản lý tập trung hơn

### Không được commit vào repo

- file private key SSH
- JWT private key
- file `.env` production

## 13. JWT private key và public key trong project này là gì

Project auth-service dùng cặp khóa JWT để ký và xác thực token.

Ý nghĩa:

- `JWT private key`: dùng để ký token
- `JWT public key`: dùng để xác thực token đã ký

Vì sao không commit private key:

- ai có private key có thể tự ký token giả
- đó là rủi ro bảo mật cực lớn

Trong project này:

- VPS giữ file key thật để app chạy
- repo chỉ giữ cấu hình đường dẫn file key

## 14. Cách đọc workflow hiện tại

### `backend-ci.yml`

Workflow này chạy khi:

- mở PR
- push vào `main`
- bấm `Run workflow` thủ công

Nó có 2 job chính:

- `go-services`
- `docker-build`

`go-services` chạy check cho:

- `auth-service`
- `email-service`
- `group-service`

`docker-build` sẽ:

- tạo file key/cert tạm cho môi trường CI
- validate `docker compose config`
- build image Docker để đảm bảo Dockerfile không lỗi

### `backend-cd.yml`

Workflow này chạy khi:

- push vào `main`
- hoặc bấm `workflow_dispatch`

Nó có 2 job chính:

- `publish-images`
- `deploy-production`

`publish-images`:

- build image từng service
- gắn tag theo commit SHA và `latest`
- push image lên `GHCR`

`deploy-production`:

- copy file compose lên VPS
- SSH vào VPS
- login `GHCR`
- pull image mới
- up lại container

## 15. Cách chuẩn bị VPS

VPS của bạn cần:

- Linux
- Docker
- Docker Compose
- user SSH có thể đăng nhập bằng key

Thư mục deploy mặc định:

```text
/opt/money-tracking
```

Chuẩn bị một lần:

```bash
mkdir -p /opt/money-tracking/auth-service/keys
mkdir -p /opt/money-tracking/deploy
```

Sau đó đặt JWT key vào:

```text
/opt/money-tracking/auth-service/keys/jwt-private.pem
/opt/money-tracking/auth-service/keys/jwt-public.pem
```

## 16. Cách kiểm tra SSH key hiện có trên máy local

Trên Windows PowerShell:

```powershell
Get-ChildItem $HOME\.ssh
```

Nếu đã có:

- `id_ed25519`
- `id_ed25519.pub`

thì bạn đã có một cặp key.

## 17. Cách tạo SSH key riêng cho GitHub Actions

Khuyến nghị: tạo riêng một key chỉ dùng cho deploy GitHub Actions.

Ví dụ:

```powershell
ssh-keygen -t ed25519 -C "github-actions-deploy" -f $HOME\.ssh\github_actions_deploy
```

Sau khi chạy, bạn sẽ có:

- private key:
  - `C:\Users\ASUS\.ssh\github_actions_deploy`
- public key:
  - `C:\Users\ASUS\.ssh\github_actions_deploy.pub`

Lợi ích của key riêng:

- nếu sau này cần thu hồi, chỉ thu hồi key này
- không ảnh hưởng key bạn đang dùng để SSH thủ công

## 18. Cách thêm public key vào VPS

Xem nội dung public key:

```powershell
Get-Content $HOME\.ssh\github_actions_deploy.pub
```

Sau đó SSH vào VPS và thêm vào:

```bash
~/.ssh/authorized_keys
```

Ví dụ:

```bash
mkdir -p ~/.ssh
chmod 700 ~/.ssh
echo "NOI_DUNG_PUBLIC_KEY" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

Ý nghĩa:

- VPS sẽ tin tất cả ai có private key tương ứng

## 19. Cách thêm private key vào GitHub

Xem nội dung private key trên máy local:

```powershell
Get-Content $HOME\.ssh\github_actions_deploy
```

Copy toàn bộ nội dung đó, rồi lên GitHub:

- `Repository`
- `Settings`
- `Secrets and variables`
- `Actions`
- `New repository secret`

Tạo secret:

```text
DEPLOY_SSH_KEY
```

Giá trị là toàn bộ private key.

Lưu ý:

- chỉ copy private key vào GitHub Secrets
- không commit file private key vào repo

## 20. Cách thêm các secrets còn lại vào GitHub

Trong repo GitHub:

- `Settings`
- `Secrets and variables`
- `Actions`

Tạo các secrets sau:

```text
DEPLOY_HOST
DEPLOY_PORT
DEPLOY_USER
DEPLOY_PATH
AUTH_DATABASE_URL
GROUP_DATABASE_URL
POSTGRES_DB
POSTGRES_USERNAME
POSTGRES_PASSWORD
GROUP_POSTGRES_DB
GROUP_POSTGRES_USERNAME
GROUP_POSTGRES_PASSWORD
REDIS_PASSWORD
RABBITMQ_USERNAME
RABBITMQ_PASSWORD
JWT_KEY_ID
JWT_PUBLIC_KEYS
JWT_REFRESH_TTL
SMTP_HOST
SMTP_PORT
SMTP_USERNAME
SMTP_PASSWORD
EMAIL_FROM
LOG_LEVEL
CORS_ALLOWED_ORIGIN_REGEX
DEPLOY_SSH_KEY
```

Với hệ thống hiện tại, giá trị mặc định vận hành nên là:

- branch deploy: `main`
- registry: `ghcr.io`
- deploy path: `/opt/money-tracking`
- image tag deploy: commit SHA

## 21. Checklist cấu hình thủ công

### Trên máy local

- kiểm tra SSH key hiện có
- hoặc tạo key mới `github_actions_deploy`
- giữ private key ở máy local
- copy private key vào `DEPLOY_SSH_KEY`

### Trên VPS

- đảm bảo SSH đăng nhập bằng key được
- đảm bảo Docker và Compose hoạt động
- tạo thư mục `/opt/money-tracking`
- tạo thư mục `/opt/money-tracking/auth-service/keys`
- thêm public key vào `authorized_keys`
- chép file JWT vào đúng chỗ

### Trên GitHub

- thêm toàn bộ `Actions Secrets`
- đảm bảo repo có quyền dùng `GITHUB_TOKEN` để push package lên GHCR

### Trong repo

- commit workflow
- commit tài liệu
- push lên GitHub

### Chạy thử

- tạo PR để test `Backend CI`
- merge vào `main` để test `Backend CD`
- hoặc dùng `workflow_dispatch`

## 22. Cách trigger CI và CD

### Trigger CI

CI chạy khi:

- bạn mở `pull request`
- hoặc push vào `main`

### Trigger CD

CD chạy khi:

- push vào `main`
- hoặc bấm `Run workflow` trong tab `Actions`

## 23. Cách debug nếu deploy fail

### Nếu fail ở CI

Kiểm tra:

- test Go có fail không
- lint có fail không
- `docker compose config` có thiếu env không
- Docker build có fail ở service nào không

### Nếu fail ở publish image

Kiểm tra:

- `GITHUB_TOKEN` có quyền push package không
- tên image/tag có đúng không
- `api-gateway` có đang build đúng root context không

### Nếu fail ở SSH

Kiểm tra:

- `DEPLOY_SSH_KEY` có đúng private key không
- public key đã thêm vào VPS chưa
- `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_PORT` có đúng không

### Nếu fail ở deploy trên VPS

Kiểm tra:

- thư mục `/opt/money-tracking` có tồn tại không
- file JWT có đúng chỗ không
- `docker login ghcr.io` có thành công không
- `docker compose pull` có kéo được image không
- `docker compose ps` có thấy container lên không

## 24. Những hiểu lầm thường gặp

### "GitHub Actions đăng nhập VPS bằng password"

Không đúng.

Hệ thống này dùng:

- SSH private key trong GitHub Secrets
- public key trên VPS

### "Repo public là lộ server"

Không đúng nếu bạn không đưa private key và secret vào repo.

### "VPS phải giữ source code để deploy"

Không cần trong mô hình hiện tại.

VPS chỉ cần:

- Docker
- Compose
- file key thật
- khả năng pull image từ GHCR

### "Public key là bí mật"

Không.

Chỉ `private key` mới là bí mật.

## 25. Kết luận ngắn gọn

Hệ thống này hoạt động theo nguyên tắc:

- GitHub Actions build
- GHCR lưu image
- VPS pull image và chạy
- SSH key dùng để GitHub Actions đăng nhập VPS an toàn

Nếu bạn nhớ được 4 ý này thì gần như bạn đã nắm được toàn bộ cơ chế triển khai hiện tại của project.
