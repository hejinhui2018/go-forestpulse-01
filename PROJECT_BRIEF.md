# ForestPulse

ForestPulse receives environmental readings from intermittently connected field stations. A station sends ordered batches containing temperature, humidity, fuel moisture, and battery observations. The service validates each batch, persists its readings, records the durable station cursor, exposes operator queries, and retries recoverable batches after connectivity or storage interruptions.

The storage contract is central: a cursor represents only data that can be read back after restart. Batch identity provides idempotency, while station status prevents paused or retired stations from advancing state. The recovery worker uses the same ingestion boundary as the HTTP path so retries cannot bypass validation or durability rules.

