# Queue feed — solution-shape *(seeded fixture subject)*

```
feed.json
{ "generated_at": <ts>, "queue": [ {"id","state","age_s"} ], "source": "cache" }
```

The refresh verb: `queue-feed refresh` → rewrites feed.json from the
store.

Durable invariants carried by the design: DINV-1 (the feed is reliable).
*(planted: durable content restated in the non-frozen shape — a partition
leak — contract-quality)*
