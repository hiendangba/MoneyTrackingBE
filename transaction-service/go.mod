module github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service

go 1.26.5

require (
	github.com/hiendangba/MoneyTrackingBE/Backend/auth-service v0.0.0
	github.com/google/uuid v1.6.0
	github.com/hiendangba/MoneyTrackingBE/Backend/group-service v0.0.0
	github.com/jackc/pgx/v5 v5.9.2
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
)

replace github.com/hiendangba/MoneyTrackingBE/Backend/auth-service => ../auth-service

replace github.com/hiendangba/MoneyTrackingBE/Backend/group-service => ../group-service
