# plat-telemetry

Telemetry collection

https://github.com/joeblew999/plat-telemetry

Announcement: https://basekick.net/blog/liftbridge-joins-basekick-labs

## Architecture

```
┌──────────┐     ┌─────────┐     ┌────────────┐     ┌─────┐     ┌─────────┐
│ Telegraf │────▶│  NATS   │────▶│ Liftbridge │────▶│ Arc │────▶│ Grafana │
│          │     │ :4222   │     │   :9292    │     │:8000│     │  :3000  │
└──────────┘     └─────────┘     └────────────┘     └─────┘     └─────────┘
```

**Ports:**
- NATS: 4222 (clients), 8222 (monitoring)
- Liftbridge: 9292
- Arc: 8000
- Grafana: 3000

---

# Src

Race Conditions solved with:

https://github.com/Basekick-Labs/liftbridge

Kafka-style message streaming in Go. Built on NATS. 

---

https://github.com/basekick-labs/arc


High-performance time-series database for Industrial IoT and Analytics. 9.47M records/sec. Racing telemetry, smart cities, mining sensors, medical devices. DuckDB SQL + Parquet + Arrow. AGPL-3.0

---

https://github.com/influxdata/telegraf
https://github.com/Basekick-Labs/telegraf

Agent for collecting, processing, aggregating, and writing metrics, logs, and other arbitrary data.

## Continuous Queries

Arc supports automatic downsampling via continuous queries - ~400x storage compression (20GB raw → 50MB aggregated).

https://docs.basekick.net/arc/data-lifecycle/continuous-queries




