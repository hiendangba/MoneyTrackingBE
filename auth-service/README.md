# auth-service

Go auth service for MoneyTracking.

## Features

- register + verify otp
- login with access/refresh cookies
- refresh token rotation
- logout with redis blacklist
- forgot password + reset password
- change password

## Environment

Copy `.env.example` and fill the values.

## Run

```powershell
& "C:\Program Files\Go\bin\go.exe" run ./cmd/api
```
