---
title: "Architecture"
---

```
Telegraf → NATS → Liftbridge → Arc → Grafana
```

| Component | Port | Purpose |
|-----------|------|---------|
| NATS | 4222 | Messaging |
| Liftbridge | 9292 | Streaming |
| Arc | 8000 | Analytics |
| Grafana | 3000 | Dashboards |
