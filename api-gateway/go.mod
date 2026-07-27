module github.com/hiendangba/MoneyTrackingBE/Backend/api-gateway

go 1.26.5

require (
	github.com/hiendangba/MoneyTrackingBE/Backend/auth-service v0.0.0
	github.com/hiendangba/MoneyTrackingBE/Backend/group-service v0.0.0
	github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service v0.0.0
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
)

replace github.com/hiendangba/MoneyTrackingBE/Backend/auth-service => ../auth-service

replace github.com/hiendangba/MoneyTrackingBE/Backend/group-service => ../group-service

replace github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service => ../transaction-service
