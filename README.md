Loosely based on https://github.com/libdns/tencentcloud

Differences:
1. No record ID is required, and it will not be returned either.
2. `SetRecords` works as `libdns` [documented](https://pkg.go.dev/github.com/libdns/libdns#RecordSetter).
