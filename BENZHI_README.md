# LockFlow

LockFlow is a Go service for coordinating two navigation lock chambers. It combines convoy admission, gate and valve commands, water-level transitions, field signals, emergency interlocks, and partitioned event recovery behind an HTTP control surface.

## Run locally

The repository vendors its Go dependencies for offline builds.

```text
go build -mod=vendor -o lockflow.exe ./cmd/lockflow
lockflow.exe -address 127.0.0.1:19696 -data var/lockflow -web web
```

Open `http://127.0.0.1:19696/operations` for dispatch operations. Chamber state, interlocks, and incident review are available from the top navigation.

## HTTP checks

`GET /healthz` reports device connectivity. Current chamber, interlock, and incident state is available through `/api/chambers`, `/api/interlocks`, and `/api/incidents`.
