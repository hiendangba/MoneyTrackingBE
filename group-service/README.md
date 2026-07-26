# group-service

`group-service` owns all group-related data and logic:

- groups
- group members
- invitations

Other services should call this service over gRPC when they need group data.

## Local run

1. Build protobuf code with `buf generate`
2. Run migrations against the group database
3. Start the service

Default gRPC address:

- `0.0.0.0:50052`

