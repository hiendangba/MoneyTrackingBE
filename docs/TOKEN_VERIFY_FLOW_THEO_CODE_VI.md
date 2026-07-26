# Giải Thích Cơ Chế Verify Token Theo Đúng Code

Tài liệu này không nói JWT theo kiểu lý thuyết chung chung. Nó bám theo đúng code đang chạy trong project `MoneyTracking`, để bạn hiểu:

- request đi như thế nào
- `api-gateway` đang làm gì với token
- `auth-service` verify token ra sao
- vì sao code lại được viết theo cách đó
- nếu muốn tối ưu hơn thì tương lai có thể đổi ở đâu

Nếu muốn hiểu bức tranh lớn trước, bạn đọc thêm file:

- [MICROSERVICE_KIEN_THUC_VI.md](C:/hoctap/Study/MoneyTracking/Backend/docs/MICROSERVICE_KIEN_THUC_VI.md)

File này tập trung vào phần hẹp hơn:

- `Frontend -> api-gateway -> auth-service`
- verify token
- claims
- CSRF
- bottleneck

## 1. Nhìn luồng tổng quát trước

Hiện tại, khi một request cần đăng nhập hoặc truy cập route protected đi vào hệ thống, luồng cơ bản là:

```text
Frontend
  -> HTTP request
api-gateway
  -> lấy token từ Header hoặc Cookie
  -> gọi gRPC sang auth-service để verify token
auth-service
  -> parse JWT
  -> kiểm tra chữ ký
  -> kiểm tra claims và rule
  -> trả claims hợp lệ về gateway
api-gateway
  -> nhét claims vào context
  -> cho request đi tiếp
```

Điểm rất quan trọng:

- `api-gateway` hiện tại không tự verify JWT local
- nó gửi token sang `auth-service` để verify

Đây là thiết kế có chủ đích, không phải vô tình viết như vậy.

## 2. File nào đang giữ vai trò gì

Bạn có thể đọc theo thứ tự này:

- [middleware.go](C:/hoctap/Study/MoneyTracking/Backend/api-gateway/internal/transport/http/middleware.go)
  - nơi gateway lấy token, gọi verify, check role, check csrf
- [server.go](C:/hoctap/Study/MoneyTracking/Backend/auth-service/internal/transport/grpc/server.go)
  - lớp gRPC nhận request từ gateway rồi gọi xuống logic auth thật
- [jwt_service.go](C:/hoctap/Study/MoneyTracking/Backend/auth-service/internal/service/jwt_service.go)
  - nơi parse và verify JWT thật sự

Hiểu vai trò:

- `middleware.go` là lớp “gác cổng”
- `server.go` là lớp “adapter gRPC”
- `jwt_service.go` mới là lớp “logic verify token thật”

Nếu bạn bị rối khi đọc code, cứ tự hỏi:

- file này đang nhận request?
- file này đang chuyển tiếp request?
- hay file này đang xử lý logic thật?

## 3. Gateway lấy token ở đâu

Ở [middleware.go](C:/hoctap/Study/MoneyTracking/Backend/api-gateway/internal/transport/http/middleware.go), hàm `extractToken(r)` làm đúng việc tên của nó:

- thử lấy Bearer token từ `Authorization`
- nếu không có thì thử lấy cookie `access_token`

Luồng của nó là:

```go
if token := extractBearerToken(r.Header.Get("Authorization")); token != "" {
    return token
}
if cookie, err := r.Cookie("access_token"); err == nil {
    return cookie.Value
}
return ""
```

Ý nghĩa:

- nếu client gửi `Authorization: Bearer ...` thì ưu tiên lấy theo cách đó
- nếu không có, gateway mới fallback sang cookie

## 4. Vì sao ưu tiên Bearer token rồi mới tới cookie

Đây không phải ngẫu nhiên.

Lý do:

- mobile client hoặc API client thường gửi Bearer token
- browser flow trong project này lại thường dùng cookie

Nếu ưu tiên Bearer token trước:

- request từ mobile/API rõ ràng hơn
- client có thể chủ động override token qua header
- tránh nhập nhằng khi cùng lúc vừa có cookie vừa có header

Hiểu đơn giản:

- `Authorization` là token mà client cố tình gửi
- cookie đôi khi là thứ browser tự gửi kèm

Vì vậy ưu tiên header trước là hợp lý hơn.

## 5. Điều gì xảy ra khi request vào route cần auth

Ở `api-gateway`, các route protected sẽ đi qua `requireAuth(...)`.

Hàm này làm 4 việc chính:

1. Lấy token
2. Nếu không có token thì trả lỗi luôn
3. Gọi sang `auth-service` để verify
4. Nếu verify thành công thì lấy claims và cho request đi tiếp

Code quan trọng là đoạn:

```go
token := extractToken(r)
if token == "" {
    writeError(w, status.Error(codes.Unauthenticated, "missing access token"), g.logger)
    return
}
```

Ý nghĩa:

- gateway không cố “đoán” auth nếu token không có
- thiếu token thì dừng ngay ở gateway

## 6. Vì sao gateway tạo timeout trước khi gọi auth-service

Ngay sau khi lấy token, gateway làm:

```go
ctx, cancel := context.WithTimeout(r.Context(), g.cfg.RequestTimeout)
defer cancel()
```

Mục đích:

- không để request treo vô hạn nếu `auth-service` chậm hoặc lỗi
- giới hạn thời gian chờ khi gọi service khác

Hiểu thực tế:

- nếu không có timeout, một request có thể bị treo rất lâu
- timeout giúp hệ thống fail sớm và dễ kiểm soát hơn

Đây là một thói quen rất quan trọng trong microservice:

- gọi service khác thì nên có timeout

## 7. Gateway đang gọi sang auth-service như thế nào

Sau khi có token, gateway gọi:

```go
resp, err := g.authClient.ValidateAccessToken(...)
```

Đây là một gRPC call.

Tức là gateway không tự parse JWT, mà gửi nguyên token cho `auth-service` và hỏi:

"Token này có hợp lệ không? Nếu hợp lệ thì user là ai?"

Điều này thể hiện một quyết định kiến trúc:

- verify token đang được tập trung ở `auth-service`

## 8. Vì sao gateway không tự verify JWT local

Vì project hiện tại đang chọn mô hình `centralized verification`.

Nghĩa là:

- `auth-service` là nơi hiểu auth sâu nhất
- `gateway` không tự giữ logic verify JWT
- `gateway` chỉ gọi sang nơi chuyên trách để kiểm tra

Lợi ích:

- logic auth tập trung
- dễ sửa policy verify ở một nơi
- dễ quản lý session/revoke/version check tập trung hơn

Nhược điểm:

- thêm một network hop
- mỗi request protected lại phải gọi `auth-service`
- nếu traffic lớn, `auth-service` dễ thành bottleneck

## 9. Sau khi auth-service verify xong thì gateway nhận gì về

Gateway không nhận về toàn bộ token. Nó chỉ nhận những claim cần để tiếp tục xử lý request.

Code tạo `AuthClaims`:

```go
claims := AuthClaims{
    UserID:         resp.GetUserId(),
    RoleID:         resp.GetRoleId(),
    RoleCode:       resp.GetRoleCode(),
    SessionID:      resp.GetSessionId(),
    SessionVersion: resp.GetSessionVersion(),
}
```

Ý nghĩa:

- `UserID`: user hiện tại là ai
- `RoleID`, `RoleCode`: role gì
- `SessionID`: đang thuộc phiên đăng nhập nào
- `SessionVersion`: phiên đó đang ở version bao nhiêu

Gateway chỉ cần từng này để:

- biết request thuộc user nào
- biết user có role gì
- truyền tiếp thông tin đó vào business logic

## 10. Vì sao gateway nhét claims vào context

Sau khi verify thành công, gateway làm:

```go
next(w, r.WithContext(context.WithValue(ctx, authClaimsKey, claims)), claims)
```

Mục đích:

- handler phía sau không phải verify lại từ đầu
- dữ liệu auth đã nằm sẵn trong context request

Hiểu như vậy:

- middleware lo xác thực
- handler business chỉ dùng kết quả xác thực

Đây là pattern rất phổ biến:

- middleware xử lý concern chung
- handler xử lý nghiệp vụ

## 11. `requireRole()` hoạt động ra sao

`requireRole(roleCode, next)` thực chất chỉ bọc thêm lên `requireAuth`.

Nó:

1. verify auth trước
2. lấy `claims.RoleCode`
3. nếu role không khớp thì trả `forbidden`

Code quan trọng:

```go
if claims.RoleCode != roleCode {
    writeError(w, status.Error(codes.PermissionDenied, "forbidden"), g.logger)
    return
}
```

Ý nghĩa:

- auth và authorization là hai bước khác nhau
- auth trả lời: “bạn là ai?”
- role check trả lời: “bạn có quyền làm việc này không?”

## 12. `requireCSRF()` là gì và khác gì với verify JWT

Đây là một chỗ rất dễ nhầm.

### Verify JWT

Verify JWT dùng để kiểm tra:

- request này đến từ user đã đăng nhập hợp lệ hay chưa

### CSRF check

CSRF dùng để chống kiểu:

- browser bị lừa gửi request nguy hiểm bằng cookie đang đăng nhập sẵn

Trong `requireCSRF()`, gateway kiểm tra:

- method có phải kiểu cần bảo vệ không (`POST`, `PATCH`, `PUT`, `DELETE`)
- `Origin` có hợp lệ không
- cookie CSRF có tồn tại không
- header `X-CSRF-Token` có khớp cookie không

Điểm rất quan trọng:

- JWT auth và CSRF không thay thế cho nhau
- chúng bảo vệ hai vấn đề khác nhau

Hiểu đơn giản:

- JWT: “bạn có phải user hợp lệ không?”
- CSRF: “request này có phải do đúng frontend của mình chủ động gửi không?”

## 13. Auth-service làm gì khi nhận `ValidateAccessToken`

Ở [server.go](C:/hoctap\Study\MoneyTracking\Backend\auth-service\internal\transport\grpc\server.go), method:

```go
func (s *Server) ValidateAccessToken(...)
```

làm rất ít việc:

- nhận access token
- gọi xuống `s.authService.ValidateAccessToken(...)`
- nếu thành công thì map claims sang response gRPC

Điều này cho thấy:

- gRPC transport không phải nơi chứa logic verify thật
- nó chỉ là lớp nhận request và trả response

Nghĩa là:

- `server.go` là lớp adapter
- logic thật nằm sâu hơn ở `JWTService`

## 14. Verify token thật sự nằm ở đâu

Phần verify JWT thật sự nằm ở:

- [jwt_service.go](C:/hoctap/Study/MoneyTracking/Backend/auth-service/internal/service/jwt_service.go)

Hàm:

```go
ParseAccessToken(tokenString string)
```

gọi tiếp:

```go
parseAndValidate(tokenString, audience, tokenType)
```

Đây là nơi quan trọng nhất của luồng verify token hiện tại.

## 15. `parseAndValidate()` kiểm tra những gì

Hàm này không chỉ “giải mã token”.

Nó kiểm tra rất nhiều thứ:

- token có đúng thuật toán `RS256` không
- token có `kid` không
- `kid` đó có map được sang đúng `public key` không
- `audience` có đúng không
- `issuer` có đúng không
- token có `exp`, `iat`, `nbf` không
- token có hết hạn chưa
- token có đúng loại không (`access` hay `refresh`)
- `roleCode` có hợp lệ cho access token không
- `sessionId` và `sessionVersion` có tồn tại không
- lifetime của token có vượt quá rule cấu hình không

Đó là lý do mình nói:

- verify token không phải chỉ là “giải mã chuỗi JWT”
- nó là kiểm tra cả chữ ký lẫn contract bảo mật

## 16. Vì sao phải check thuật toán `RS256`

Trong code:

```go
if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
}
```

Mục tiêu:

- không cho token dùng thuật toán khác len vào
- tránh trường hợp attacker lợi dụng algorithm confusion

Hiểu đơn giản:

- hệ thống đã chốt chỉ tin token ký bằng `RS256`
- token nào khai báo thuật toán khác thì reject luôn

## 17. `kid` là gì và vì sao phải check

`kid` là `key id`.

Nó nằm trong header của JWT, không phải phần claim.

Ý nghĩa:

- hệ thống có thể có nhiều public key
- token phải nói nó được ký bằng key nào
- service verify sẽ lấy đúng public key tương ứng để kiểm tra

Trong code:

```go
kid, ok := token.Header["kid"].(string)
...
publicKey, ok := s.publicKeys[kid]
```

Nếu không có `kid`, hoặc `kid` không map ra key nào:

- token bị reject

Lý do kiến trúc:

- hỗ trợ `key rotation`

Tức là:

- hôm nay bạn dùng key A
- mai bạn đổi sang key B
- hệ thống vẫn có thể nhận diện token nào được ký bằng key nào trong giai đoạn chuyển tiếp

## 18. Vì sao phải có `JWT_PUBLIC_KEYS`

`JWT_PUBLIC_KEYS` là danh sách key công khai mà hệ thống chấp nhận để verify token.

Nó giúp:

- verify được nhiều `kid`
- giữ được khả năng xoay khóa

Ví dụ tư duy:

- key mới dùng để ký token mới
- key cũ vẫn giữ lại một thời gian để verify token cũ chưa hết hạn

Đó là lý do token flow an toàn hơn khi có cơ chế key ring thay vì chỉ một public key duy nhất hard-code.

## 19. Auth-service verify bằng private key hay public key

Đây là chỗ rất dễ nhầm.

### Khi ký token

`auth-service` dùng:

- `private key`

### Khi verify token

`auth-service` dùng:

- `public key`

Điều này hoàn toàn đúng với mô hình `RS256`.

Tức là:

- private key để ký
- public key để verify

Private key không dùng để verify token ở luồng này.

## 20. Vì sao phải tách access token và refresh token

Project tách:

- `access token`
- `refresh token`

vì hai loại này có mục đích khác nhau.

### Access token

Dùng để:

- truy cập API

Đặc điểm:

- sống ngắn
- phải chứa role/claim cần cho authorize

### Refresh token

Dùng để:

- xin cấp access token mới

Đặc điểm:

- sống lâu hơn
- không được dùng như access token

Trong code, `validateTokenType(claims, want)` đảm bảo:

- access token không bị dùng như refresh token
- refresh token không bị dùng như access token

## 21. `audience` và `issuer` để làm gì

### `issuer`

`issuer` trả lời:

- token này do ai phát hành

Trong project:

- issuer dự kiến là `auth-service`

### `audience`

`audience` trả lời:

- token này được cấp cho mục đích nào

Ví dụ:

- access token cho API
- refresh token cho refresh flow

Nếu không check hai thứ này, hệ thống có thể:

- tin nhầm token do nơi khác phát hành
- hoặc dùng sai loại token cho sai mục đích

## 22. `sessionId` và `sessionVersion` để làm gì

Đây là phần rất thực chiến.

`sessionId` giúp:

- biết token thuộc về phiên đăng nhập nào

`sessionVersion` giúp:

- vô hiệu hóa token cũ khi có biến cố như đổi password hoặc revoke session

Hiểu ngắn gọn:

- token không chỉ cần chữ ký đúng
- nó còn phải khớp với trạng thái phiên đăng nhập mà hệ thống đang chấp nhận

Đây là lý do auth của project này không dừng ở “JWT hợp lệ là xong”.

## 23. Vì sao code hiện tại chọn `centralized verification`

Hiện tại hệ thống chọn:

- gateway không tự verify local
- mọi verify đi qua `auth-service`

Lý do hợp lý:

- logic auth tập trung
- đổi luật verify chỉ sửa ở một nơi
- dễ quản `sessionVersion`, revoke, rule bảo mật
- gateway không phải giữ logic JWT quá sâu

Ở giai đoạn đầu hoặc hệ thống chưa quá lớn, đây là một quyết định dễ hiểu và dễ maintain.

## 24. Nhược điểm của cách hiện tại là gì

Nhược điểm chính:

- mỗi request protected thêm một network hop
- `auth-service` chịu tải verify cho nhiều request
- nếu traffic lớn, nó dễ thành bottleneck

Ví dụ:

- request tới `group-service`
- trước khi chạm business logic, gateway phải gọi thêm `auth-service`
- như vậy thời gian phản hồi tăng thêm một đoạn chờ

Nhanh hơn hay chậm hơn ở đây không đến từ “JWT nặng”, mà đến từ:

- thêm một lần gọi service qua network

## 25. Nếu tối ưu hơn trong tương lai thì sẽ đổi ở đâu

Hướng cải tiến tự nhiên là:

- `auth-service` vẫn là nơi phát token
- `api-gateway` tự verify access token bằng `public key` hoặc JWKS cache
- chỉ gọi thêm auth/state layer khi cần kiểm tra sâu như revoke, session version động

Khi đó bạn bớt được bước:

- `gateway -> auth-service -> gateway`

trong các request chỉ cần verify access token thông thường.

## 26. Ví dụ browser flow và Bearer flow

### Browser flow

Browser thường:

- có cookie `access_token`
- có cookie CSRF
- gửi thêm header `X-CSRF-Token` ở request mutation

Gateway sẽ:

- lấy token từ cookie nếu không có Bearer header
- verify token
- check CSRF nếu method là `POST`, `PATCH`, `PUT`, `DELETE`

### Bearer flow

Mobile hoặc API client thường:

- gửi `Authorization: Bearer <token>`

Gateway sẽ:

- ưu tiên lấy token từ header
- verify token
- không phụ thuộc vào browser cookie flow như CSRF

## 27. Đọc code như thế nào để không bị rối

Khi đọc các file auth flow, nên đọc như sau:

### Bước 1: đọc `middleware.go`

Tự hỏi:

- request vào đây thì lấy token ra sao
- verify ở đâu
- role check ở đâu
- csrf check ở đâu

### Bước 2: đọc `server.go`

Tự hỏi:

- gateway gọi method gRPC nào
- method đó map request/response ra sao
- có logic auth thật ở đây không

Kết luận đúng ở file này là:

- phần lớn chỉ là adapter

### Bước 3: đọc `jwt_service.go`

Tự hỏi:

- token được parse như thế nào
- check rule nào trước, rule nào sau
- vì sao rule đó tồn tại

Kết luận đúng ở file này là:

- đây mới là nơi “verify token thật sự”

## 28. Hiểu lầm thường gặp

### “Gateway đang giải mã token local”

Không đúng trong code hiện tại.

Gateway đang:

- lấy token
- gửi sang `auth-service`
- dùng kết quả verify trả về

### “Verify token chỉ là giải mã chuỗi JWT”

Không đúng.

Nó còn gồm:

- verify chữ ký
- check thuật toán
- check `kid`
- check `aud`, `iss`
- check `exp`, `iat`, `nbf`
- check loại token
- check contract claim

### “Có public key rồi thì gateway đang tự verify”

Chưa đúng với repo hiện tại.

Đó là hướng tối ưu có thể làm sau, nhưng hiện tại gateway vẫn verify tập trung qua `auth-service`.

## 29. Nhớ nhanh 5 ý

Nếu bạn chỉ muốn nhớ nhanh:

1. `api-gateway` hiện không tự verify JWT local.
2. Gateway lấy token từ Bearer header trước, rồi mới fallback sang cookie.
3. `auth-service` mới là nơi parse và verify token thật sự.
4. `RS256` nghĩa là private key ký, public key verify.
5. Cách hiện tại dễ quản auth hơn, nhưng có thể thành bottleneck nếu hệ thống lớn lên.

## 30. Kết luận ngắn gọn

Code hiện tại đang đi theo triết lý:

- gateway chỉ làm lớp gác cổng và điều phối
- auth-service mới là nơi hiểu token sâu nhất

Vì vậy khi bạn thấy gateway “đụng vào token”, hãy hiểu đúng là:

- nó đang lấy token ra khỏi request
- gửi token đi verify
- nhận claims hợp lệ về để dùng tiếp

Chứ gateway hiện tại chưa phải nơi tự giải mã và verify JWT local.

Đó là lý do code được viết như bây giờ.
