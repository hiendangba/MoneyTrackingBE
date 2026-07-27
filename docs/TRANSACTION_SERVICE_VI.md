# Transaction service

## Trách nhiệm

`transaction-service` là nguồn dữ liệu duy nhất cho:

- danh mục thu/chi;
- giao dịch cá nhân;
- giao dịch nhóm, người đã trả và phần được chia;
- các lần thanh toán bù trừ (`settlement`);
- số dư và gợi ý thanh toán trong nhóm.

Service public gRPC tại cổng `50053`. API Gateway chuyển các RPC này thành REST
và luôn lấy `user_id` từ access token, không tin định danh do client gửi.

## Luồng tạo giao dịch nhóm

1. Kiểm tra người tạo là thành viên qua `group-service.GetMembership`.
2. Lấy group và danh sách thành viên qua `group-service`.
3. Kiểm tra mọi payer/participant đều thuộc nhóm.
4. Lấy fullname/email của các user qua `auth-service.Me`.
5. Tính `share_amount`, kiểm tra tổng payment và tổng share bằng tổng giao dịch.
6. Lưu transaction, payments và splits trong một PostgreSQL transaction.

Tên group, fullname và email được lưu dạng snapshot. Vì vậy lịch sử, báo cáo và
tính số dư không phải gọi lại auth/group; hai upstream chỉ cần online ở thời
điểm ghi hoặc kiểm tra quyền.

## Quy tắc tiền

- API dùng decimal string, ví dụ `"125000.00"`.
- Domain dùng integer minor unit (`int64`), không dùng `float`.
- `fixed`: tổng `split_value` phải bằng tổng giao dịch.
- `ratio`: hệ số có tối đa 6 chữ số thập phân; phần dư làm tròn được phân phối
  ổn định theo largest remainder.
- Tổng payments phải bằng tổng giao dịch.
- Với expense: `net = paid - share`.
- Với income: `net = share - received`.
- Settlement từ A đến B làm số dư A tăng và số dư B giảm.
- Tổng số dư của một group/currency luôn phải bằng 0.

## REST endpoints

- `GET|POST /api/categories`
- `GET|POST /api/transactions/personal`
- `GET|DELETE /api/transactions/personal/{id}`
- `GET|POST /api/groups/{id}/transactions`
- `GET|DELETE /api/groups/{id}/transactions/{transaction_id}`
- `GET|POST /api/groups/{id}/settlements`
- `GET /api/groups/{id}/balances?currency=VND`

Các endpoint ghi trên web yêu cầu CSRF token giống các endpoint hiện hữu.

## Chạy local

1. Sao chép `.env.example` thành `.env` và thay các giá trị `change-me`.
2. Sinh key JWT theo hướng dẫn của auth-service.
3. Chạy `docker compose up --build`.

Migrations của transaction database được chạy tự động bởi
`transaction-migrate` trước khi service khởi động.
