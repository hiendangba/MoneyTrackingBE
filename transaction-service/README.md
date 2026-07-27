# transaction-service

`transaction-service` owns financial data and calculations:

- expense categories;
- personal income and expenses;
- group transactions, payers, fixed/ratio splits;
- settlements, balances, and settlement suggestions.

The service calls `group-service` to validate membership and `auth-service` to
resolve user profiles when writing. Names and emails are stored as immutable
snapshots, so historical reads and reports do not depend on those upstream
services.

Amounts in protobuf/JSON are decimal strings (for example `"125000.00"`).
Domain calculations use integer minor units to avoid floating-point errors.

Default gRPC address: `0.0.0.0:50053`.
