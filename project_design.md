# ForestPulse project design

ForestPulse serves environmental field stations that reconnect intermittently and upload ordered batches of readings. A station batch moves through validation, durable persistence, cursor advancement, alert evaluation, and operator query. The batch record, readings, station metadata, and cursor form one recovery contract: a cursor may only describe readings that remain available after restart.

The repository separates transport, ingestion orchestration, station state, storage, recovery, alert policy, health, query, and audit concerns. The local storage implementation writes one atomic snapshot and exposes consistency checks so edge deployments can recover without a database service. Normal tests cover domain validation, durable batch persistence, sequence enforcement, and the HTTP workflow.

The smoke executable creates temporary state, registers a station, submits a two-reading batch over HTTP, and reads the resulting cursor. It is intended as a fast operational check in addition to the full test, vet, and build commands.

